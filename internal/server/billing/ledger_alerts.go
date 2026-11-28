package billing

import (
	"context"
	"strconv"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/ent/ledgeraccount"
	"sub2api-panel/ent/ledgeralert"
	"sub2api-panel/ent/ledgerhour"
	"sub2api-panel/ent/ledgersample"
	"sub2api-panel/ent/ledgerwallet"
	"sub2api-panel/internal/server/core"
)

// 告警求值、开启与恢复，以及推断充值事件。规则见 docs/decisions/0008-上游账本对账与站内告警.md。

const (
	ledgerAlertOpen      = "open"
	ledgerAlertResolved  = "resolved"
	ledgerRunwayWindowHr = 6
)

// WireLedger 把账本接到余额采样回调：采样落库后立即求值该账号的告警。
// 在 router.WireHubs 里调用；必须在 StartBalanceMonitor 之后。
func WireLedger(s *core.Server) {
	if s == nil || s.Balances == nil || s.Database == nil {
		return
	}
	s.LedgerCards = func(ctx context.Context, ids []int64) (map[int64]core.LedgerCardSummary, error) {
		return LedgerCardsFor(ctx, s, ids)
	}
	s.Balances.SetHooks(core.BalanceHooks{
		OnSample: func(ctx context.Context, id int64, config core.BalanceConfig, view core.AccountBalanceView) {
			if RecordLedgerSample(ctx, s, id, config, view) == nil {
				_ = EvaluateLedgerAccount(ctx, s, id, time.Now())
			}
		},
		OnFailure: func(ctx context.Context, id int64, config core.BalanceConfig, _ error) {
			if RecordLedgerFailure(ctx, s, id, config) == nil {
				_ = EvaluateLedgerAccount(ctx, s, id, time.Now())
			}
		},
	})
}

// setLedgerAlert 条件成立时开启（同一账号同一类型已有 open 则只刷新数值），条件消失时恢复。
func setLedgerAlert(ctx context.Context, db *ent.Client, accountID, walletID int64, kind, severity string, active bool, value float64, now int64) error {
	open, err := db.LedgerAlert.Query().Where(ledgeralert.AccountIDEQ(accountID), ledgeralert.KindEQ(kind), ledgeralert.StatusEQ(ledgerAlertOpen)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return err
	}
	switch {
	case active && open == nil:
		create := db.LedgerAlert.Create().SetAccountID(accountID).SetKind(kind).SetSeverity(severity).
			SetStatus(ledgerAlertOpen).SetOpenedAt(now).SetValue(value)
		if walletID > 0 {
			create.SetWalletID(walletID)
		}
		if err := create.Exec(ctx); err != nil && !ent.IsConstraintError(err) {
			return err
		}
	case active:
		return open.Update().SetValue(value).SetSeverity(severity).Exec(ctx)
	case open != nil:
		return open.Update().SetStatus(ledgerAlertResolved).SetResolvedAt(now).Exec(ctx)
	}
	return nil
}

type ledgerTodayTotals struct {
	expected  float64
	actual    float64
	hasActual bool
}

func ledgerTodayOf(ctx context.Context, db *ent.Client, accountID int64, now time.Time, loc *time.Location) (ledgerTodayTotals, error) {
	var t ledgerTodayTotals
	rows, err := db.LedgerHour.Query().Where(ledgerhour.AccountIDEQ(accountID), ledgerhour.BucketGTE(ledgerDayStart(now, loc).Unix())).All(ctx)
	if err != nil {
		return t, err
	}
	for _, r := range rows {
		t.expected += r.ExpectedBalanceCostUsd + r.ExpectedSubscriptionCostUsd
		if r.ActualCostUsd != nil {
			t.actual += *r.ActualCostUsd
			t.hasActual = true
		}
	}
	return t, nil
}

func ledgerRecentHourlySpend(ctx context.Context, db *ent.Client, accountID int64, now time.Time, loc *time.Location) (float64, error) {
	since := ledgerHourStart(now.Add(-ledgerRunwayWindowHr*time.Hour), loc)
	rows, err := db.LedgerHour.Query().Where(ledgerhour.AccountIDEQ(accountID), ledgerhour.BucketGTE(since)).All(ctx)
	if err != nil {
		return 0, err
	}
	var sum float64
	for _, r := range rows {
		sum += r.ExpectedBalanceCostUsd + r.ExpectedSubscriptionCostUsd
	}
	return sum / ledgerRunwayWindowHr, nil
}

// ledgerConsecutiveFailures 统计最近连续的 failure 样本数。
func ledgerConsecutiveFailures(ctx context.Context, db *ent.Client, accountID int64, limit int) (int, error) {
	rows, err := db.LedgerSample.Query().Where(ledgersample.AccountIDEQ(accountID)).
		Order(ent.Desc(ledgersample.FieldLastSeen), ent.Desc(ledgersample.FieldID)).Limit(limit).All(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rows {
		if r.Kind != ledgerKindFailure {
			break
		}
		n++
	}
	return n, nil
}

// EvaluateLedgerAccount 对一个账号求值全部告警，并在需要时写入推断充值事件。
func EvaluateLedgerAccount(ctx context.Context, s *core.Server, accountID int64, now time.Time) error {
	if s == nil || s.Database == nil {
		return nil
	}
	db, loc, nowUnix := s.Database, s.UsageZone(), now.Unix()
	settings := LoadLedgerSettings(ctx, db)
	acct, err := db.LedgerAccount.Query().Where(ledgeraccount.AccountIDEQ(accountID), ledgeraccount.DeletedAtIsNil()).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil
		}
		return err
	}
	wallet, err := db.LedgerWallet.Query().Where(ledgerwallet.IDEQ(int(acct.WalletID))).Only(ctx)
	if err != nil {
		return err
	}
	failures, err := ledgerConsecutiveFailures(ctx, db, accountID, settings.FailureThreshold+1)
	if err != nil {
		return err
	}
	if err := setLedgerAlert(ctx, db, accountID, acct.WalletID, "sample_failure", "warn", failures >= settings.FailureThreshold, float64(failures), nowUnix); err != nil {
		return err
	}
	samples, err := db.LedgerSample.Query().Where(ledgersample.AccountIDEQ(accountID), ledgersample.KindNEQ(ledgerKindFailure)).
		Order(ent.Desc(ledgersample.FieldLastSeen), ent.Desc(ledgersample.FieldID)).Limit(2).All(ctx)
	if err != nil || len(samples) == 0 {
		return err
	}
	cur := samples[0]
	var prev *ent.LedgerSample
	if len(samples) > 1 && samples[1].WalletID == cur.WalletID {
		prev = samples[1]
	}
	if err := evaluateLedgerReadings(ctx, db, acct, wallet, cur, prev, settings, nowUnix); err != nil {
		return err
	}
	return evaluateLedgerCost(ctx, db, acct, cur, settings, now, loc)
}

func evaluateLedgerReadings(ctx context.Context, db *ent.Client, acct *ent.LedgerAccount, wallet *ent.LedgerWallet, cur, prev *ent.LedgerSample, settings LedgerSettings, now int64) error {
	var prevMultiplier *float64
	var drop, spend float64
	if prev != nil {
		prevMultiplier = prev.Multiplier
		if d, reset, ok := ledgerUsedDelta(prev.KeyUsedUsd, cur.KeyUsedUsd); ok && !reset {
			spend = d
		} else if d, reset, ok := ledgerUsedDelta(prev.WalletUsedUsd, cur.WalletUsedUsd); ok && !reset {
			spend = d
		}
		if prev.BalanceUsd != nil && cur.BalanceUsd != nil {
			drop = *prev.BalanceUsd - *cur.BalanceUsd
		}
	}
	shared := prev != nil && ledgerSharedWalletSuspect(drop, spend)
	if err := setLedgerAlert(ctx, db, acct.AccountID, acct.WalletID, "shared_wallet_suspect", "warn", shared, drop-spend, now); err != nil {
		return err
	}
	changed := prev != nil && ledgerMultiplierChanged(prevMultiplier, cur.Multiplier)
	value := 0.0
	if changed {
		value = *cur.Multiplier
	}
	if err := setLedgerAlert(ctx, db, acct.AccountID, acct.WalletID, "multiplier_change", "info", changed, value, now); err != nil {
		return err
	}
	missing := wallet.Provider == "sub2api" && cur.Multiplier == nil
	if err := setLedgerAlert(ctx, db, acct.AccountID, acct.WalletID, "billing_missing", "info", missing, 0, now); err != nil {
		return err
	}
	low := cur.BalanceUsd != nil && *cur.BalanceUsd <= 0
	balance := 0.0
	if cur.BalanceUsd != nil {
		balance = *cur.BalanceUsd
	}
	if err := setLedgerAlert(ctx, db, acct.AccountID, acct.WalletID, "low_balance", "bad", low, balance, now); err != nil {
		return err
	}
	if prev == nil || prev.BalanceUsd == nil || cur.BalanceUsd == nil {
		return nil
	}
	amount, ok := ledgerInferRecharge(*prev.BalanceUsd, *cur.BalanceUsd, spend)
	if !ok {
		return nil
	}
	status := "confirmed"
	if shared || !wallet.AutoRecharge {
		status = "pending"
	}
	err := db.LedgerEvent.Create().SetWalletID(acct.WalletID).SetAccountID(acct.AccountID).SetKind("recharge_inferred").
		SetAmountUsd(amount).SetOccurredAt(cur.FirstSeen).SetSource("inferred").
		SetExternalID("sample:" + strconv.Itoa(cur.ID)).SetStatus(status).SetCreated(now).SetUpdated(now).Exec(ctx)
	if err != nil && !ent.IsConstraintError(err) {
		return err
	}
	return nil
}

func evaluateLedgerCost(ctx context.Context, db *ent.Client, acct *ent.LedgerAccount, cur *ent.LedgerSample, settings LedgerSettings, now time.Time, loc *time.Location) error {
	today, err := ledgerTodayOf(ctx, db, acct.AccountID, now, loc)
	if err != nil {
		return err
	}
	var actual *float64
	if today.hasActual {
		actual = &today.actual
	}
	usd, pct := ledgerDeviation(actual, today.expected)
	breached := ledgerDeviationBreached(usd, pct, settings.DeviationMinUSD, settings.DeviationPct)
	value := 0.0
	if pct != nil {
		value = *pct
	}
	if err := setLedgerAlert(ctx, db, acct.AccountID, acct.WalletID, "deviation", "warn", breached, value, now.Unix()); err != nil {
		return err
	}
	hourly, err := ledgerRecentHourlySpend(ctx, db, acct.AccountID, now, loc)
	if err != nil {
		return err
	}
	var runway *float64
	if cur.BalanceUsd != nil {
		runway = ledgerRunwayHours(*cur.BalanceUsd, hourly)
	}
	short := runway != nil && *runway < settings.RunwayHours
	hours := 0.0
	if runway != nil {
		hours = *runway
	}
	return setLedgerAlert(ctx, db, acct.AccountID, acct.WalletID, "runway", "warn", short, hours, now.Unix())
}
