package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/ent/ledgeraccount"
	"sub2api-panel/ent/ledgeralert"
	"sub2api-panel/ent/ledgerday"
	"sub2api-panel/ent/ledgerhour"
	"sub2api-panel/ent/ledgersample"
	"sub2api-panel/ent/ledgerwallet"
	"sub2api-panel/ent/runtimesetting"
	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

// 账本接口：路由、汇总、单账号序列与设置。事件与告警接口见 ledger_alert_handlers.go。
// 响应结构只用 ledger_types.go 里定死的白名单类型。

func (h *handler) registerLedgerRoutes(mux *http.ServeMux) {
	h.s.Route(mux, "GET /api/admin/ledger/summary", true, h.ledgerSummary)
	h.s.Route(mux, "GET /api/admin/ledger/accounts/{id}", true, h.ledgerAccountSeries)
	h.s.Route(mux, "POST /api/admin/ledger/accounts/{id}/events", true, h.ledgerEventCreate)
	h.s.Route(mux, "PATCH /api/admin/ledger/accounts/{id}/events/{event}", true, h.ledgerEventUpdate)
	h.s.Route(mux, "DELETE /api/admin/ledger/accounts/{id}/events/{event}", true, h.ledgerEventVoid)
	h.s.Route(mux, "GET /api/admin/ledger/alerts", true, h.ledgerAlertList)
	h.s.Route(mux, "GET /api/admin/ledger/alerts/unread-count", true, h.ledgerAlertUnread)
	h.s.Route(mux, "POST /api/admin/ledger/alerts/read-all", true, h.ledgerAlertReadAll)
	h.s.Route(mux, "POST /api/admin/ledger/alerts/{id}/read", true, h.ledgerAlertRead)
	h.s.Route(mux, "POST /api/admin/ledger/alerts/{id}/dismiss", true, h.ledgerAlertDismiss)
	h.s.Route(mux, "GET /api/admin/ledger/settings", true, h.ledgerSettingsGet)
	h.s.Route(mux, "PUT /api/admin/ledger/settings", true, h.ledgerSettingsPut)
	h.s.Route(mux, "POST /api/admin/ledger/probe", true, h.ledgerProbe)
}

func (h *handler) ledgerDB() (*ent.Client, error) {
	if h.s.Database == nil {
		return nil, platform.Fail(503, "service_unavailable")
	}
	return h.s.Database, nil
}

type ledgerCtx struct {
	db       *ent.Client
	loc      *time.Location
	now      time.Time
	settings LedgerSettings
	official bool
}

func (h *handler) newLedgerCtx(ctx context.Context, db *ent.Client) ledgerCtx {
	return ledgerCtx{db: db, loc: h.s.UsageZone(), now: time.Now(), settings: LoadLedgerSettings(ctx, db), official: h.s.OfficialDB != nil}
}

func ledgerReconcilable(provider string) bool {
	return provider == "sub2api" || provider == "newapi" || provider == "oneapi"
}

func ledgerTime(unix int64, loc *time.Location) string {
	return time.Unix(unix, 0).In(loc).Format(time.RFC3339)
}

func ledgerRevenueToday(ctx context.Context, lc ledgerCtx, accountID int64) (float64, error) {
	rows, err := lc.db.LedgerHour.Query().Where(ledgerhour.AccountIDEQ(accountID), ledgerhour.BucketGTE(ledgerDayStart(lc.now, lc.loc).Unix())).All(ctx)
	if err != nil {
		return 0, err
	}
	var sum float64
	for _, r := range rows {
		sum += r.RevenueUsd
	}
	return sum, nil
}

// ledgerCard 拼一个账号的卡片摘要。open 是该账号当前开启的告警。
func ledgerCard(ctx context.Context, lc ledgerCtx, acct *ent.LedgerAccount, wallet *ent.LedgerWallet, open []*ent.LedgerAlert) (LedgerCardSummary, error) {
	card := LedgerCardSummary{Currency: "USD", Granularity: "day", Alerts: []LedgerAlertBadge{}}
	if lc.official {
		card.Granularity = "hour"
	}
	if wallet != nil {
		provider := wallet.Provider
		card.Provider = &provider
		card.Reconcile = lc.official && ledgerReconcilable(provider)
	}
	for _, al := range open {
		card.Alerts = append(card.Alerts, LedgerAlertBadge{Kind: al.Kind, Severity: al.Severity, Label: ledgerAlertLabel(al.Kind, al.Value)})
	}
	last, err := lc.db.LedgerSample.Query().Where(ledgersample.AccountIDEQ(acct.AccountID)).
		Order(ent.Desc(ledgersample.FieldLastSeen), ent.Desc(ledgersample.FieldID)).First(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return card, err
	}
	if last != nil {
		at := ledgerTime(last.LastSeen, lc.loc)
		card.SampledAt = &at
		card.Stale = lc.now.Unix()-last.LastSeen > 2*ledgerHourSeconds
	}
	samples, err := lc.db.LedgerSample.Query().Where(ledgersample.AccountIDEQ(acct.AccountID), ledgersample.KindNEQ(ledgerKindFailure)).
		Order(ent.Desc(ledgersample.FieldLastSeen), ent.Desc(ledgersample.FieldID)).Limit(2).All(ctx)
	if err != nil {
		return card, err
	}
	if len(samples) > 0 {
		card.BalanceUSD, card.Multiplier = samples[0].BalanceUsd, samples[0].Multiplier
		if len(samples) > 1 && ledgerMultiplierChanged(samples[1].Multiplier, samples[0].Multiplier) {
			card.PreviousMultiplier = samples[1].Multiplier
		}
	}
	today, err := ledgerTodayOf(ctx, lc.db, acct.AccountID, lc.now, lc.loc)
	if err != nil {
		return card, err
	}
	card.ExpectedTodayUSD = today.expected
	if today.hasActual {
		actual := today.actual
		card.ActualTodayUSD = &actual
	}
	card.Estimated = !today.hasActual
	card.DeviationUSD, card.DeviationPct = ledgerDeviation(card.ActualTodayUSD, today.expected)
	if card.BalanceUSD != nil {
		hourly, err := ledgerRecentHourlySpend(ctx, lc.db, acct.AccountID, lc.now, lc.loc)
		if err != nil {
			return card, err
		}
		card.RunwayHours = ledgerRunwayHours(*card.BalanceUSD, hourly)
	}
	if lc.settings.ShowProfit {
		revenue, err := ledgerRevenueToday(ctx, lc, acct.AccountID)
		if err != nil {
			return card, err
		}
		profit, _ := ledgerProfit(revenue, card.ActualTodayUSD, today.expected)
		card.RevenueTodayUSD, card.ProfitTodayUSD = &revenue, &profit
	}
	return card, nil
}

func (h *handler) ledgerSummary(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	db, err := h.ledgerDB()
	if err != nil {
		return err
	}
	ctx := r.Context()
	lc := h.newLedgerCtx(ctx, db)
	accounts, err := db.LedgerAccount.Query().Where(ledgeraccount.DeletedAtIsNil()).
		Order(ent.Asc(ledgeraccount.FieldAccountID)).Limit(core.BalanceConfigLimit).All(ctx)
	if err != nil {
		return err
	}
	walletRows, err := db.LedgerWallet.Query().All(ctx)
	if err != nil {
		return err
	}
	wallets := make(map[int64]*ent.LedgerWallet, len(walletRows))
	for _, row := range walletRows {
		wallets[int64(row.ID)] = row
	}
	openRows, err := db.LedgerAlert.Query().Where(ledgeralert.StatusEQ(ledgerAlertOpen)).All(ctx)
	if err != nil {
		return err
	}
	open := map[int64][]*ent.LedgerAlert{}
	for _, row := range openRows {
		open[row.AccountID] = append(open[row.AccountID], row)
	}
	view := LedgerSummaryView{ShowProfit: lc.settings.ShowProfit, Accounts: make([]LedgerAccountCard, 0, len(accounts))}
	var revenue, profit float64
	for _, acct := range accounts {
		card, err := ledgerCard(ctx, lc, acct, wallets[acct.WalletID], open[acct.AccountID])
		if err != nil {
			return err
		}
		view.Accounts = append(view.Accounts, LedgerAccountCard{AccountID: acct.AccountID, LedgerCardSummary: card})
		if card.BalanceUSD != nil {
			view.Totals.BalanceUSD += *card.BalanceUSD
		}
		if card.ActualTodayUSD != nil {
			view.Totals.UpstreamCostTodayUSD += *card.ActualTodayUSD
		} else {
			view.Totals.UpstreamCostTodayUSD += card.ExpectedTodayUSD
		}
		if card.RevenueTodayUSD != nil && card.ProfitTodayUSD != nil {
			revenue += *card.RevenueTodayUSD
			profit += *card.ProfitTodayUSD
		}
		for _, al := range card.Alerts {
			if al.Severity == "warn" || al.Severity == "bad" {
				view.Totals.AttentionAccounts++
				break
			}
		}
	}
	if lc.settings.ShowProfit {
		view.Totals.RevenueTodayUSD, view.Totals.ProfitTodayUSD = &revenue, &profit
	}
	return platform.Send(w, 200, view, nil)
}

func parseLedgerTime(raw string, loc *time.Location) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, true
	}
	if t, err := time.ParseInLocation("2006-01-02", raw, loc); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func (h *handler) ledgerAccountOf(ctx context.Context, db *ent.Client, id int64) (*ent.LedgerAccount, error) {
	acct, err := db.LedgerAccount.Query().Where(ledgeraccount.AccountIDEQ(id)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, platform.Fail(404, "not_found")
	}
	return acct, err
}

func (h *handler) ledgerAccountSeries(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	db, err := h.ledgerDB()
	if err != nil {
		return err
	}
	ctx := r.Context()
	lc := h.newLedgerCtx(ctx, db)
	// 账本记录在首次采样时才建立（ledger_record.go）；未接入的账号返回空视图（tracked=false），前端据此显示接入引导。
	acct, err := db.LedgerAccount.Query().Where(ledgeraccount.AccountIDEQ(id)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return err
	}
	tracked := acct != nil
	if !tracked {
		acct = &ent.LedgerAccount{AccountID: id}
	}
	q := r.URL.Query()
	gran := q.Get("granularity")
	if gran == "" {
		gran = "hour"
	}
	if gran != "hour" && gran != "day" {
		return platform.Fail(400, "invalid_input")
	}
	end, span := lc.now, 24*time.Hour
	maxSpan := 7 * 24 * time.Hour
	if gran == "day" {
		span, maxSpan = 30*24*time.Hour, 366*24*time.Hour
	}
	if raw := q.Get("end"); raw != "" {
		t, ok := parseLedgerTime(raw, lc.loc)
		if !ok {
			return platform.Fail(400, "invalid_input")
		}
		end = t
		if len(raw) == 10 {
			end = t.AddDate(0, 0, 1).Add(-time.Second)
		}
	}
	start := end.Add(-span)
	if raw := q.Get("start"); raw != "" {
		t, ok := parseLedgerTime(raw, lc.loc)
		if !ok {
			return platform.Fail(400, "invalid_input")
		}
		start = t
	}
	if end.Before(start) || end.Sub(start) > maxSpan {
		return platform.Fail(400, "invalid_input")
	}
	wallet, err := db.LedgerWallet.Query().Where(ledgerwallet.IDEQ(int(acct.WalletID))).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return err
	}
	open, err := db.LedgerAlert.Query().Where(ledgeralert.AccountIDEQ(id), ledgeralert.StatusEQ(ledgerAlertOpen)).All(ctx)
	if err != nil {
		return err
	}
	summary, err := ledgerCard(ctx, lc, acct, wallet, open)
	if err != nil {
		return err
	}
	view := LedgerAccountSeriesView{AccountID: id, Tracked: tracked, Granularity: gran, ShowProfit: lc.settings.ShowProfit, Summary: summary, Points: []LedgerPointView{}, Events: []LedgerEventView{}}
	point := func(bucket, requests int64, revenue, expBal, expSub float64, actual *float64, source string) LedgerPointView {
		p := LedgerPointView{Bucket: ledgerTime(bucket, lc.loc), Requests: requests, ExpectedBalanceCostUSD: expBal, ExpectedSubscriptionCostUSD: expSub, ActualCostUSD: actual}
		if lc.settings.ShowProfit {
			p.RevenueUSD = &revenue
		}
		if source != "" {
			p.ActualSource = &source
		}
		return p
	}
	if gran == "hour" {
		rows, err := db.LedgerHour.Query().Where(ledgerhour.AccountIDEQ(id), ledgerhour.BucketGTE(ledgerHourStart(start, lc.loc)), ledgerhour.BucketLTE(end.Unix())).
			Order(ent.Asc(ledgerhour.FieldBucket)).All(ctx)
		if err != nil {
			return err
		}
		for _, row := range rows {
			view.Points = append(view.Points, point(row.Bucket, row.Requests, row.RevenueUsd, row.ExpectedBalanceCostUsd, row.ExpectedSubscriptionCostUsd, row.ActualCostUsd, row.ActualSource))
		}
	} else {
		rows, err := db.LedgerDay.Query().Where(ledgerday.AccountIDEQ(id), ledgerday.BucketGTE(ledgerDayStart(start, lc.loc).Unix()), ledgerday.BucketLTE(end.Unix())).
			Order(ent.Asc(ledgerday.FieldBucket)).All(ctx)
		if err != nil {
			return err
		}
		for _, row := range rows {
			view.Points = append(view.Points, point(row.Bucket, row.Requests, row.RevenueUsd, row.ExpectedBalanceCostUsd, row.ExpectedSubscriptionCostUsd, row.ActualCostUsd, row.ActualSource))
		}
	}
	if tracked {
		events, err := ledgerEventViews(ctx, db, acct.WalletID, lc.loc)
		if err != nil {
			return err
		}
		view.Events = events
	}
	if gran == "day" && !lc.official && len(view.Points) == 0 {
		if points := h.ledgerDailyFallback(ctx, a.Tokens.Access, id, start, end, lc.loc, lc.settings.ShowProfit); points != nil {
			view.Points = points
		}
	}
	view.Connection = LedgerConnectionView{}
	if _, config, err := h.s.FindBalanceConfig(ctx, id); err == nil {
		if config.Provider != "" {
			provider := config.Provider
			view.Connection.Provider = &provider
		}
		view.Connection.BaseURL = config.BaseURL
		view.Connection.APIKeyConfigured = config.Key != ""
		view.Connection.AccessTokenConfigured = config.AccessToken != ""
	}
	return platform.Send(w, 200, view, nil)
}

func validLedgerSettings(s LedgerSettings) bool {
	return s.DeviationPct >= 0 && s.DeviationPct <= 1000 &&
		s.DeviationMinUSD >= 0 && s.DeviationMinUSD <= 1_000_000 &&
		s.RunwayHours >= 0 && s.RunwayHours <= 8760 &&
		s.FailureThreshold >= 1 && s.FailureThreshold <= 100 &&
		s.HourRetentionDays >= 7 && s.HourRetentionDays <= 3650 &&
		s.SampleRetentionDays >= 1 && s.SampleRetentionDays <= 365
}

func (h *handler) ledgerSettingsGet(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	db, err := h.ledgerDB()
	if err != nil {
		return err
	}
	return platform.Send(w, 200, LoadLedgerSettings(r.Context(), db), nil)
}

func (h *handler) ledgerSettingsPut(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	db, err := h.ledgerDB()
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[LedgerSettings](r)
	if err != nil {
		return err
	}
	if !validLedgerSettings(input) {
		return platform.Fail(400, "invalid_input")
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	ctx, now := r.Context(), time.Now().Unix()
	row, err := db.RuntimeSetting.Query().Where(runtimesetting.KeyEQ(LedgerSettingsKey)).Only(ctx)
	switch {
	case ent.IsNotFound(err):
		err = db.RuntimeSetting.Create().SetKey(LedgerSettingsKey).SetValue(string(raw)).SetUpdated(now).Exec(ctx)
		if ent.IsConstraintError(err) {
			return platform.Fail(409, "invalid_input")
		}
	case err == nil:
		err = row.Update().SetValue(string(raw)).SetUpdated(now).Exec(ctx)
	}
	return platform.Send(w, 200, input, err)
}
