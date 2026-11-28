package billing

import (
	"context"
	"encoding/json"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/ent/ledgeraccount"
	"sub2api-panel/ent/ledgeralert"
	"sub2api-panel/ent/ledgerday"
	"sub2api-panel/ent/ledgerevent"
	"sub2api-panel/ent/ledgerhour"
	"sub2api-panel/ent/ledgersample"
	"sub2api-panel/ent/runtimesetting"
	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
	"sub2api-panel/official/usagelog"
)

// 小时汇总、日汇总与过期清理。规则见 docs/decisions/0008-上游账本对账与站内告警.md。

const (
	LedgerSettingsKey       = "ledger_settings"
	ledgerRollupEvery       = 10 * time.Minute
	ledgerRollupWindow      = 3 * time.Hour
	ledgerAlertRetentionDay = 90
	ledgerSourceKeyDelta    = "usage_delta"
	ledgerSourceBalance     = "balance_delta"
)

// LoadLedgerSettings 读取 runtime_settings 的 ledger_settings 键，缺失或损坏时返回默认值。
func LoadLedgerSettings(ctx context.Context, db *ent.Client) LedgerSettings {
	settings := DefaultLedgerSettings()
	if db == nil {
		return settings
	}
	row, err := db.RuntimeSetting.Query().Where(runtimesetting.KeyEQ(LedgerSettingsKey)).Only(ctx)
	if err != nil {
		return settings
	}
	loaded := settings
	if json.Unmarshal([]byte(row.Value), &loaded) != nil || loaded.HourRetentionDays < 1 || loaded.SampleRetentionDays < 1 {
		return settings
	}
	return loaded
}

// StartLedgerRollup 启动后台汇总：立即跑一次，之后每 10 分钟重算最近 3 小时，每小时清理一次过期数据。
func StartLedgerRollup(ctx context.Context, s *core.Server) {
	if s == nil || s.Database == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(ledgerRollupEvery)
		defer ticker.Stop()
		var lastCleanup time.Time
		for {
			runCtx, end := platform.BackgroundFlow(ctx, "汇总上游账本")
			_ = RollupLedger(runCtx, s, time.Now())
			if time.Since(lastCleanup) >= time.Hour {
				if CleanupLedger(runCtx, s, time.Now()) == nil {
					lastCleanup = time.Now()
				}
			}
			end()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

type ledgerHourAgg struct {
	requests int64
	revenue  float64
	expBal   float64
	expSub   float64
}

func ledgerHourStart(t time.Time, loc *time.Location) int64 {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, loc).Unix()
}

func ledgerDayStart(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// RollupLedger 重算最近 3 小时的小时汇总与实际花费，并刷新今天和昨天的日汇总。
// 未接官方只读库时本站侧无从聚合，直接跳过。
func RollupLedger(ctx context.Context, s *core.Server, now time.Time) error {
	if s == nil || s.Database == nil || s.OfficialDB == nil {
		return nil
	}
	loc := s.UsageZone()
	window := time.Unix(ledgerHourStart(now.Add(-ledgerRollupWindow), loc), 0)
	if err := rollupLedgerUsage(ctx, s, window, loc); err != nil {
		return err
	}
	if err := rollupLedgerActual(ctx, s, window.Unix(), loc); err != nil {
		return err
	}
	for _, day := range []time.Time{ledgerDayStart(now, loc), ledgerDayStart(now.AddDate(0, 0, -1), loc)} {
		if err := rollupLedgerDay(ctx, s.Database, day, loc); err != nil {
			return err
		}
	}
	return nil
}

// rollupLedgerUsage 把官方 usage_logs 按账号与小时桶聚合，覆盖写入本站侧字段。
func rollupLedgerUsage(ctx context.Context, s *core.Server, window time.Time, loc *time.Location) error {
	var rows []struct {
		AccountID             int64     `json:"account_id"`
		BillingType           int8      `json:"billing_type"`
		ActualCost            float64   `json:"actual_cost"`
		TotalCost             float64   `json:"total_cost"`
		AccountStatsCost      *float64  `json:"account_stats_cost"`
		AccountRateMultiplier *float64  `json:"account_rate_multiplier"`
		CreatedAt             time.Time `json:"created_at"`
	}
	if err := s.OfficialDB.UsageLog.Query().Where(usagelog.CreatedAtGTE(window)).
		Select(usagelog.FieldAccountID, usagelog.FieldBillingType, usagelog.FieldActualCost, usagelog.FieldTotalCost,
			usagelog.FieldAccountStatsCost, usagelog.FieldAccountRateMultiplier, usagelog.FieldCreatedAt).
		Scan(ctx, &rows); err != nil {
		return err
	}
	agg := map[int64]map[int64]*ledgerHourAgg{}
	for _, r := range rows {
		buckets := agg[r.AccountID]
		if buckets == nil {
			buckets = map[int64]*ledgerHourAgg{}
			agg[r.AccountID] = buckets
		}
		b := ledgerHourStart(r.CreatedAt, loc)
		cell := buckets[b]
		if cell == nil {
			cell = &ledgerHourAgg{}
			buckets[b] = cell
		}
		cell.requests++
		cost := r.TotalCost
		if r.AccountStatsCost != nil {
			cost = *r.AccountStatsCost
		}
		multiplier := 1.0
		if r.AccountRateMultiplier != nil {
			multiplier = *r.AccountRateMultiplier
		}
		if r.BillingType == 0 {
			cell.revenue += r.ActualCost
			cell.expBal += cost * multiplier
		} else {
			cell.expSub += cost * multiplier
		}
	}
	now := time.Now().Unix()
	for accountID, buckets := range agg {
		if accountID <= 0 {
			continue
		}
		for bucket, cell := range buckets {
			row, err := s.Database.LedgerHour.Query().Where(ledgerhour.AccountIDEQ(accountID), ledgerhour.BucketEQ(bucket)).Only(ctx)
			if ent.IsNotFound(err) {
				err = s.Database.LedgerHour.Create().SetAccountID(accountID).SetBucket(bucket).SetRequests(cell.requests).
					SetRevenueUsd(cell.revenue).SetExpectedBalanceCostUsd(cell.expBal).SetExpectedSubscriptionCostUsd(cell.expSub).
					SetUpdated(now).Exec(ctx)
				if ent.IsConstraintError(err) {
					continue
				}
			}
			if err != nil {
				return err
			}
			if row != nil {
				if err := row.Update().SetRequests(cell.requests).SetRevenueUsd(cell.revenue).SetExpectedBalanceCostUsd(cell.expBal).
					SetExpectedSubscriptionCostUsd(cell.expSub).SetUpdated(now).Exec(ctx); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ledgerSpendOf 取一对相邻采样之间的实际花费：优先 Key 累计已用，其次仅在钱包只有这一个账号时用钱包累计已用；
// 累计值回退时用“余额变化 + 期间充值”兜底。
func ledgerSpendOf(prev, cur *ent.LedgerSample, walletUsable bool, recharged float64) (float64, string, bool) {
	delta, reset, ok := ledgerUsedDelta(prev.KeyUsedUsd, cur.KeyUsedUsd)
	if !ok && walletUsable {
		delta, reset, ok = ledgerUsedDelta(prev.WalletUsedUsd, cur.WalletUsedUsd)
	}
	if !ok {
		return 0, "", false
	}
	if !reset {
		return delta, ledgerSourceKeyDelta, true
	}
	if prev.BalanceUsd == nil || cur.BalanceUsd == nil {
		return 0, "", false
	}
	return ledgerResetSpend(*prev.BalanceUsd, *cur.BalanceUsd, recharged), ledgerSourceBalance, true
}

// rollupLedgerActual 把相邻采样间的实际花费按预计花费比例摊到小时桶。
// 只写入窗口内的小时；窗口前的一条采样作锚点，保证每个被写的小时拿到全部重叠区间。
func rollupLedgerActual(ctx context.Context, s *core.Server, windowUnix int64, loc *time.Location) error {
	db := s.Database
	accounts, err := db.LedgerAccount.Query().Where(ledgeraccount.DeletedAtIsNil()).All(ctx)
	if err != nil {
		return err
	}
	walletSize := map[int64]int{}
	for _, a := range accounts {
		walletSize[a.WalletID]++
	}
	for _, a := range accounts {
		samples, err := db.LedgerSample.Query().
			Where(ledgersample.AccountIDEQ(a.AccountID), ledgersample.KindNEQ(ledgerKindFailure), ledgersample.LastSeenGTE(windowUnix)).
			Order(ent.Asc(ledgersample.FieldLastSeen)).All(ctx)
		if err != nil {
			return err
		}
		anchor, err := db.LedgerSample.Query().
			Where(ledgersample.AccountIDEQ(a.AccountID), ledgersample.KindNEQ(ledgerKindFailure), ledgersample.LastSeenLT(windowUnix)).
			Order(ent.Desc(ledgersample.FieldLastSeen)).First(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return err
		}
		if anchor != nil {
			samples = append([]*ent.LedgerSample{anchor}, samples...)
		}
		if len(samples) < 2 {
			continue
		}
		actuals, sources := map[int64]float64{}, map[int64]string{}
		for i := 1; i < len(samples); i++ {
			prev, cur := samples[i-1], samples[i]
			from, to := prev.LastSeen, cur.FirstSeen
			if to <= from || prev.WalletID != cur.WalletID {
				continue
			}
			recharged := ledgerRechargedBetween(ctx, db, cur.WalletID, from, to)
			spend, source, ok := ledgerSpendOf(prev, cur, walletSize[cur.WalletID] <= 1, recharged)
			if !ok {
				continue
			}
			weights, err := ledgerHourWeights(ctx, db, a.AccountID, from, to, loc)
			if err != nil {
				return err
			}
			spread := ledgerSpread(spend, from, to, weights)
			for bucket, value := range spread {
				if bucket >= windowUnix {
					actuals[bucket] += value
					sources[bucket] = source
				}
			}
			// 窗口内没有任何花费的小时也要落一个 0，才能和“没采样”区分。
			for _, w := range weights {
				if _, seen := actuals[w.Bucket]; !seen && w.Bucket >= windowUnix && w.Bucket+ledgerHourSeconds <= cur.FirstSeen {
					actuals[w.Bucket], sources[w.Bucket] = 0, source
				}
			}
		}
		for bucket, value := range actuals {
			if err := saveLedgerActual(ctx, db, a.AccountID, bucket, value, sources[bucket]); err != nil {
				return err
			}
		}
	}
	return nil
}

func ledgerRechargedBetween(ctx context.Context, db *ent.Client, walletID, from, to int64) float64 {
	rows, err := db.LedgerEvent.Query().Where(
		ledgerevent.WalletIDEQ(walletID), ledgerevent.StatusEQ("confirmed"),
		ledgerevent.KindIn("recharge", "recharge_inferred"),
		ledgerevent.OccurredAtGTE(from), ledgerevent.OccurredAtLTE(to)).All(ctx)
	if err != nil {
		return 0
	}
	var sum float64
	for _, r := range rows {
		sum += r.AmountUsd
	}
	return sum
}

// ledgerHourWeights 取区间覆盖的各小时桶的预计花费（余额 + 订阅），缺行按 0。
func ledgerHourWeights(ctx context.Context, db *ent.Client, accountID, from, to int64, loc *time.Location) ([]ledgerHourWeight, error) {
	first := ledgerHourStart(time.Unix(from, 0), loc)
	rows, err := db.LedgerHour.Query().Where(ledgerhour.AccountIDEQ(accountID), ledgerhour.BucketGTE(first), ledgerhour.BucketLT(to)).All(ctx)
	if err != nil {
		return nil, err
	}
	expected := map[int64]float64{}
	for _, r := range rows {
		expected[r.Bucket] = r.ExpectedBalanceCostUsd + r.ExpectedSubscriptionCostUsd
	}
	var out []ledgerHourWeight
	for b := first; b < to; b += ledgerHourSeconds {
		out = append(out, ledgerHourWeight{Bucket: b, Expected: expected[b]})
	}
	return out, nil
}

func saveLedgerActual(ctx context.Context, db *ent.Client, accountID, bucket int64, value float64, source string) error {
	now := time.Now().Unix()
	row, err := db.LedgerHour.Query().Where(ledgerhour.AccountIDEQ(accountID), ledgerhour.BucketEQ(bucket)).Only(ctx)
	if ent.IsNotFound(err) {
		err = db.LedgerHour.Create().SetAccountID(accountID).SetBucket(bucket).SetActualCostUsd(value).SetActualSource(source).SetUpdated(now).Exec(ctx)
		if ent.IsConstraintError(err) {
			return nil
		}
		return err
	}
	if err != nil {
		return err
	}
	return row.Update().SetActualCostUsd(value).SetActualSource(source).SetUpdated(now).Exec(ctx)
}

// rollupLedgerDay 把一天内的小时行按账号汇总写入日表；某账号全天没有任何实际花费读数时，日表实际花费留空。
func rollupLedgerDay(ctx context.Context, db *ent.Client, day time.Time, loc *time.Location) error {
	start := day.Unix()
	end := ledgerDayStart(day.AddDate(0, 0, 1), loc).Unix()
	rows, err := db.LedgerHour.Query().Where(ledgerhour.BucketGTE(start), ledgerhour.BucketLT(end)).All(ctx)
	if err != nil {
		return err
	}
	type dayAgg struct {
		ledgerHourAgg
		actual    float64
		hasActual bool
		source    string
	}
	byAccount := map[int64]*dayAgg{}
	for _, r := range rows {
		cell := byAccount[r.AccountID]
		if cell == nil {
			cell = &dayAgg{}
			byAccount[r.AccountID] = cell
		}
		cell.requests += r.Requests
		cell.revenue += r.RevenueUsd
		cell.expBal += r.ExpectedBalanceCostUsd
		cell.expSub += r.ExpectedSubscriptionCostUsd
		if r.ActualCostUsd != nil {
			cell.actual += *r.ActualCostUsd
			cell.hasActual = true
			if cell.source == "" {
				cell.source = r.ActualSource
			}
		}
	}
	now := time.Now().Unix()
	for accountID, cell := range byAccount {
		row, err := db.LedgerDay.Query().Where(ledgerday.AccountIDEQ(accountID), ledgerday.BucketEQ(start)).Only(ctx)
		if ent.IsNotFound(err) {
			create := db.LedgerDay.Create().SetAccountID(accountID).SetBucket(start).SetRequests(cell.requests).SetRevenueUsd(cell.revenue).
				SetExpectedBalanceCostUsd(cell.expBal).SetExpectedSubscriptionCostUsd(cell.expSub).SetActualSource(cell.source).SetUpdated(now)
			if cell.hasActual {
				create.SetActualCostUsd(cell.actual)
			}
			if err := create.Exec(ctx); err != nil && !ent.IsConstraintError(err) {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		update := row.Update().SetRequests(cell.requests).SetRevenueUsd(cell.revenue).SetExpectedBalanceCostUsd(cell.expBal).
			SetExpectedSubscriptionCostUsd(cell.expSub).SetActualSource(cell.source).SetUpdated(now)
		if cell.hasActual {
			update.SetActualCostUsd(cell.actual)
		} else {
			update.ClearActualCostUsd()
		}
		if err := update.Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

// CleanupLedger 只删除过期数据：小时行、采样行与已结束的告警；事件、日表、钱包与账号永久保留。
func CleanupLedger(ctx context.Context, s *core.Server, now time.Time) error {
	if s == nil || s.Database == nil {
		return nil
	}
	settings := LoadLedgerSettings(ctx, s.Database)
	day := 24 * time.Hour
	if _, err := s.Database.LedgerHour.Delete().Where(ledgerhour.BucketLT(now.Add(-time.Duration(settings.HourRetentionDays) * day).Unix())).Exec(ctx); err != nil {
		return err
	}
	if _, err := s.Database.LedgerSample.Delete().Where(ledgersample.LastSeenLT(now.Add(-time.Duration(settings.SampleRetentionDays) * day).Unix())).Exec(ctx); err != nil {
		return err
	}
	_, err := s.Database.LedgerAlert.Delete().Where(
		ledgeralert.StatusNEQ("open"),
		ledgeralert.ResolvedAtLT(now.Add(-ledgerAlertRetentionDay*day).Unix())).Exec(ctx)
	return err
}
