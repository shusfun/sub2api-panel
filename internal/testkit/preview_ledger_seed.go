package testkit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/ent/ledgeraccount"
	"sub2api-panel/ent/ledgeralert"
	"sub2api-panel/ent/ledgerday"
	"sub2api-panel/ent/ledgerevent"
	"sub2api-panel/ent/ledgerhour"
	"sub2api-panel/ent/ledgersample"
	"sub2api-panel/ent/ledgerwallet"
)

// 预览模式的账本历史样本。采样要求 https 公网地址，夹具上游提供不了，
// 所以这里直接用 Ent 往七张账本表灌一份确定的历史。只由 TestBrowserFixture 在预览启动时调用，正式启动路径不碰。
//
// 账号从夹具实际返回的账号里挑：有目录文件时取 f.previewAccounts() 里 type == "apikey" 的账号，
// 没有目录文件时夹具只有账号 5 且没有 type，不是 apikey，整体跳过。名称与平台取夹具的值。
//
// 可重复运行：钱包 identity 统一用 previewLedgerPrefix 前缀。每次先删掉该前缀钱包及其账号、样本、事件、告警、小时与日汇总，
// 再按当前夹具重新写入，所以换目录文件后不会留下旧账号的数据。

const previewLedgerPrefix = "preview-ledger-"

type previewLedgerCandidate struct {
	id       int64
	name     string
	platform string
}

type previewLedgerPlan struct {
	baseCost   float64 // 每小时预计花费（美元）
	skew       float64 // 实际花费 / 预计花费
	balance    float64
	multiplier *float64
	walletUsed bool
}

type previewLedgerEvent struct {
	idx    int
	kind   string
	status string
	note   string
	amount float64
	ago    int64 // 距今秒数
}

type previewLedgerAlert struct {
	idx      int
	kind     string
	severity string
	status   string
	value    float64
	openAgo  int64
	doneAgo  int64 // 非 0 表示已恢复，距今秒数
}

type previewLedgerScenario struct {
	key      string
	title    string
	provider string
	origin   string
	perUSD   float64
	user     int64
	plans    []previewLedgerPlan
	events   []previewLedgerEvent
	alerts   []previewLedgerAlert
}

func previewLedgerScenarios() []previewLedgerScenario {
	mult := func(v float64) *float64 { return &v }
	return []previewLedgerScenario{
		{key: "shared", title: "New API 共用钱包", provider: "newapi", origin: "newapi.example.test", perUSD: 500000, user: 12,
			plans: []previewLedgerPlan{{0.80, 1.05, 42.6, mult(1), true}, {0.50, 1.00, 42.6, mult(1), true}},
			events: []previewLedgerEvent{
				{0, "recharge", "confirmed", "手动补录", 100, 5 * 86400},
				{0, "recharge_inferred", "pending", "", 50, 7200},
			},
			alerts: []previewLedgerAlert{{0, "shared_wallet_suspect", "warn", "open", 1.8, 1700, 0}}},
		{key: "low", title: "余额不足", provider: "newapi", origin: "low.example.test", perUSD: 500000,
			plans: []previewLedgerPlan{{1.20, 1.00, 0.35, mult(1), false}},
			alerts: []previewLedgerAlert{
				{0, "low_balance", "bad", "open", 0.35, 900, 0},
				{0, "runway", "warn", "open", 1, 880, 0},
			}},
		{key: "deviation", title: "偏差", provider: "newapi", origin: "drift.example.test", perUSD: 500000,
			plans:  []previewLedgerPlan{{0.90, 1.40, 31.2, mult(1), false}},
			alerts: []previewLedgerAlert{{0, "deviation", "warn", "open", 40, 1800, 0}}},
		{key: "sub2api", title: "Sub2API Key 用量", provider: "sub2api", origin: "relay.example.test",
			plans:  []previewLedgerPlan{{0.60, 1.00, 18.4, mult(1.2), false}},
			events: []previewLedgerEvent{{0, "quota_raised", "confirmed", "", 20, 3 * 86400}},
			alerts: []previewLedgerAlert{{0, "multiplier_change", "info", "resolved", 1.2, 2 * 86400, 86400}}},
	}
}

func previewLedgerInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	}
	return 0, false
}

// previewLedgerCandidates 取夹具实际返回的 apikey 账号。
func previewLedgerCandidates(t *testing.T, f *fixture) []previewLedgerCandidate {
	t.Helper()
	if f.preview == nil {
		t.Logf("preview ledger: no catalog file; the default fixture only has account 5 and it is not type=apikey, skipping the ledger seed")
		return nil
	}
	var out []previewLedgerCandidate
	for _, row := range f.previewAccounts() {
		if kind, _ := row["type"].(string); kind != "apikey" {
			continue
		}
		id, ok := previewLedgerInt64(row["id"])
		if !ok || id < 1 {
			continue
		}
		name, _ := row["name"].(string)
		plat, _ := row["platform"].(string)
		out = append(out, previewLedgerCandidate{id: id, name: name, platform: plat})
	}
	return out
}

// clearPreviewLedger 只删除 previewLedgerPrefix 钱包名下的数据，不碰真实采样产生的钱包。
func clearPreviewLedger(ctx context.Context, db *ent.Client) error {
	wallets, err := db.LedgerWallet.Query().Where(ledgerwallet.IdentityHasPrefix(previewLedgerPrefix)).All(ctx)
	if err != nil || len(wallets) == 0 {
		return err
	}
	walletIDs := make([]int64, 0, len(wallets))
	ids := make([]int, 0, len(wallets))
	for _, w := range wallets {
		walletIDs = append(walletIDs, int64(w.ID))
		ids = append(ids, w.ID)
	}
	accounts, err := db.LedgerAccount.Query().Where(ledgeraccount.WalletIDIn(walletIDs...)).All(ctx)
	if err != nil {
		return err
	}
	accountIDs := make([]int64, 0, len(accounts))
	for _, a := range accounts {
		accountIDs = append(accountIDs, a.AccountID)
	}
	if len(accountIDs) > 0 {
		if _, err := db.LedgerHour.Delete().Where(ledgerhour.AccountIDIn(accountIDs...)).Exec(ctx); err != nil {
			return err
		}
		if _, err := db.LedgerDay.Delete().Where(ledgerday.AccountIDIn(accountIDs...)).Exec(ctx); err != nil {
			return err
		}
		if _, err := db.LedgerAlert.Delete().Where(ledgeralert.AccountIDIn(accountIDs...)).Exec(ctx); err != nil {
			return err
		}
	}
	if _, err := db.LedgerSample.Delete().Where(ledgersample.WalletIDIn(walletIDs...)).Exec(ctx); err != nil {
		return err
	}
	if _, err := db.LedgerEvent.Delete().Where(ledgerevent.WalletIDIn(walletIDs...)).Exec(ctx); err != nil {
		return err
	}
	if _, err := db.LedgerAccount.Delete().Where(ledgeraccount.WalletIDIn(walletIDs...)).Exec(ctx); err != nil {
		return err
	}
	_, err = db.LedgerWallet.Delete().Where(ledgerwallet.IDIn(ids...)).Exec(ctx)
	return err
}

func seedPreviewLedger(t *testing.T, f *fixture) {
	t.Helper()
	if f == nil || f.panel == nil || f.panel.Database == nil {
		return
	}
	app, ctx, db := f.panel, context.Background(), f.panel.Database
	candidates := previewLedgerCandidates(t, f)
	if len(candidates) == 0 {
		return
	}
	if err := clearPreviewLedger(ctx, db); err != nil {
		t.Fatalf("clear preview ledger: %v", err)
	}
	// 已被真实采样占用的账号不覆盖。
	free := candidates[:0:0]
	for _, c := range candidates {
		if n, err := db.LedgerAccount.Query().Where(ledgeraccount.AccountIDEQ(c.id)).Count(ctx); err != nil || n > 0 {
			t.Logf("preview ledger: account %d already has real ledger data, skipped", c.id)
			continue
		}
		free = append(free, c)
	}
	loc, now := app.UsageZone(), time.Now()
	nowUnix := now.Unix()
	hourStart := func(tm time.Time) int64 {
		tm = tm.In(loc)
		return time.Date(tm.Year(), tm.Month(), tm.Day(), tm.Hour(), 0, 0, 0, loc).Unix()
	}
	dayStart := func(tm time.Time) int64 {
		tm = tm.In(loc)
		return time.Date(tm.Year(), tm.Month(), tm.Day(), 0, 0, 0, 0, loc).Unix()
	}
	for _, sc := range previewLedgerScenarios() {
		if len(free) < len(sc.plans) {
			t.Logf("preview ledger: scenario %q needs %d apikey account(s), only %d left, skipped", sc.title, len(sc.plans), len(free))
			continue
		}
		picked := free[:len(sc.plans)]
		free = free[len(sc.plans):]
		wallet, err := db.LedgerWallet.Create().SetIdentity(previewLedgerPrefix + sc.key).SetProvider(sc.provider).SetOrigin(sc.origin).
			SetQuotaPerUsd(sc.perUSD).SetUpstreamUserID(sc.user).SetCreated(nowUnix).SetUpdated(nowUnix).Save(ctx)
		if err != nil {
			t.Fatalf("seed ledger wallet: %v", err)
		}
		walletID := int64(wallet.ID)
		for i, plan := range sc.plans {
			acct := picked[i]
			if err := db.LedgerAccount.Create().SetAccountID(acct.id).SetWalletID(walletID).SetName(acct.name).SetPlatform(acct.platform).
				SetKeyFingerprint(fmt.Sprintf("%s%s-%d", previewLedgerPrefix, sc.key, i)).SetCreated(nowUnix).SetUpdated(nowUnix).Exec(ctx); err != nil {
				t.Fatalf("seed ledger account: %v", err)
			}
			newSample := func() *ent.LedgerSampleCreate {
				return db.LedgerSample.Create().SetWalletID(walletID).SetAccountID(acct.id).SetKind("key").SetCurrency("USD").SetNillableMultiplier(plan.multiplier)
			}
			keyUsed := 120.0 + float64(acct.id)
			prevBalance := plan.balance + plan.baseCost*plan.skew
			if err := newSample().SetFirstSeen(nowUnix - 7200).SetLastSeen(nowUnix - 3600).SetBalanceUsd(prevBalance).SetKeyUsedUsd(keyUsed - plan.baseCost*plan.skew).Exec(ctx); err != nil {
				t.Fatalf("seed ledger sample: %v", err)
			}
			latest := newSample().SetFirstSeen(nowUnix - 3500).SetLastSeen(nowUnix - 60).SetBalanceUsd(plan.balance).SetKeyUsedUsd(keyUsed)
			if plan.walletUsed {
				latest.SetWalletUsedUsd(300)
			}
			if err := latest.Exec(ctx); err != nil {
				t.Fatalf("seed ledger sample: %v", err)
			}
			for h := 47; h >= 0; h-- {
				wave := 1 + 0.25*float64((h*7+int(acct.id))%5-2)/2
				expected := plan.baseCost * wave
				if err := db.LedgerHour.Create().SetAccountID(acct.id).SetBucket(hourStart(now.Add(-time.Duration(h) * time.Hour))).
					SetRequests(int64(40 + (h*13+int(acct.id)*5)%60)).SetRevenueUsd(expected * 1.3).
					SetExpectedBalanceCostUsd(expected * 0.8).SetExpectedSubscriptionCostUsd(expected * 0.2).
					SetActualCostUsd(expected * plan.skew).SetActualSource("usage_delta").SetUpdated(nowUnix).Exec(ctx); err != nil {
					t.Fatalf("seed ledger hour: %v", err)
				}
			}
			for d := 13; d >= 0; d-- {
				expected := plan.baseCost * 24 * (1 + 0.1*float64((d*3+int(acct.id))%4-1))
				if err := db.LedgerDay.Create().SetAccountID(acct.id).SetBucket(dayStart(now.AddDate(0, 0, -d))).
					SetRequests(int64(900 + (d*37)%300)).SetRevenueUsd(expected * 1.3).
					SetExpectedBalanceCostUsd(expected * 0.8).SetExpectedSubscriptionCostUsd(expected * 0.2).
					SetActualCostUsd(expected * plan.skew).SetActualSource("usage_delta").SetUpdated(nowUnix).Exec(ctx); err != nil {
					t.Fatalf("seed ledger day: %v", err)
				}
			}
		}
		for n, e := range sc.events {
			// 来源取正式代码会写的值：推断充值为 inferred（ledger_alerts.go），其余为 manual（ledger_alert_handlers.go）。
			source := "manual"
			if e.kind == "recharge_inferred" {
				source = "inferred"
			}
			if err := db.LedgerEvent.Create().SetWalletID(walletID).SetAccountID(picked[e.idx].id).SetKind(e.kind).SetAmountUsd(e.amount).
				SetOccurredAt(nowUnix - e.ago).SetSource(source).SetExternalID(fmt.Sprintf("%s%s:%d", previewLedgerPrefix, sc.key, n)).
				SetStatus(e.status).SetNote(e.note).SetCreated(nowUnix).SetUpdated(nowUnix).Exec(ctx); err != nil {
				t.Fatalf("seed ledger event: %v", err)
			}
		}
		for _, a := range sc.alerts {
			create := db.LedgerAlert.Create().SetAccountID(picked[a.idx].id).SetWalletID(walletID).SetKind(a.kind).SetSeverity(a.severity).
				SetStatus(a.status).SetValue(a.value).SetOpenedAt(nowUnix - a.openAgo)
			if a.doneAgo != 0 {
				create.SetResolvedAt(nowUnix - a.doneAgo).SetReadAt(nowUnix - a.doneAgo)
			}
			if err := create.Exec(ctx); err != nil {
				t.Fatalf("seed ledger alert: %v", err)
			}
		}
		t.Logf("preview ledger: scenario %q -> accounts %v", sc.title, previewLedgerIDs(picked))
	}
}

func previewLedgerIDs(list []previewLedgerCandidate) []int64 {
	out := make([]int64, 0, len(list))
	for _, c := range list {
		out = append(out, c.id)
	}
	return out
}
