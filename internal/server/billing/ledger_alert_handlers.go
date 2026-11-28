package billing

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/ent/ledgeraccount"
	"sub2api-panel/ent/ledgeralert"
	"sub2api-panel/ent/ledgerevent"
	"sub2api-panel/internal/platform"
	"sub2api-panel/internal/server/core"
)

// 账本事件与站内告警接口。

const ledgerAlertListLimit = 200

var ledgerManualKinds = map[string]bool{"recharge": true, "manual_adjust": true, "refund": true}

func ledgerAlertLabel(kind string, value float64) string {
	switch kind {
	case "deviation":
		return fmt.Sprintf("偏差 %+.0f%%", value)
	case "runway":
		return fmt.Sprintf("约可撑 %.0f 小时", value)
	case "sample_failure":
		return "采样失败"
	case "shared_wallet_suspect":
		return "疑似共用钱包"
	case "multiplier_change":
		return "上游倍率变化"
	case "low_balance":
		return "余额不足"
	case "billing_missing":
		return "上游未提供倍率"
	}
	return kind
}

func ledgerEventView(row *ent.LedgerEvent, loc *time.Location) LedgerEventView {
	return LedgerEventView{ID: int64(row.ID), Kind: row.Kind, AmountUSD: row.AmountUsd, OccurredAt: ledgerTime(row.OccurredAt, loc), Source: row.Source, Status: row.Status, Note: row.Note}
}

func ledgerEventViews(ctx context.Context, db *ent.Client, walletID int64, loc *time.Location) ([]LedgerEventView, error) {
	rows, err := db.LedgerEvent.Query().Where(ledgerevent.WalletIDEQ(walletID)).
		Order(ent.Desc(ledgerevent.FieldOccurredAt), ent.Desc(ledgerevent.FieldID)).Limit(100).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LedgerEventView, 0, len(rows))
	for _, row := range rows {
		out = append(out, ledgerEventView(row, loc))
	}
	return out, nil
}

func validLedgerAmount(v *float64) bool {
	return v != nil && *v != 0 && !math.IsNaN(*v) && math.Abs(*v) <= 1e9
}

func (h *handler) ledgerEventCreate(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return err
	}
	db, err := h.ledgerDB()
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[LedgerEventInput](r)
	if err != nil {
		return err
	}
	status := input.Status
	if status == "" {
		status = "confirmed"
	}
	if !ledgerManualKinds[input.Kind] || !validLedgerAmount(input.AmountUSD) || len(input.Note) > 255 ||
		(status != "confirmed" && status != "pending") {
		return platform.Fail(400, "invalid_input")
	}
	ctx, loc, now := r.Context(), h.s.UsageZone(), time.Now()
	acct, err := h.ledgerAccountOf(ctx, db, id)
	if err != nil {
		return err
	}
	occurred := now
	if input.OccurredAt != "" {
		t, ok := parseLedgerTime(input.OccurredAt, loc)
		if !ok {
			return platform.Fail(400, "invalid_input")
		}
		occurred = t
	}
	row, err := db.LedgerEvent.Create().SetWalletID(acct.WalletID).SetAccountID(acct.AccountID).SetKind(input.Kind).
		SetAmountUsd(*input.AmountUSD).SetOccurredAt(occurred.Unix()).SetSource("manual").SetStatus(status).
		SetNote(input.Note).SetCreated(now.Unix()).SetUpdated(now.Unix()).Save(ctx)
	if err != nil {
		return err
	}
	platform.WriteJSON(w, 201, ledgerEventView(row, loc))
	return nil
}

// ledgerEventOf 取账号所在钱包下的事件，事件不属于该钱包时按不存在处理。
func (h *handler) ledgerEventOf(r *http.Request, db *ent.Client) (*ent.LedgerAccount, *ent.LedgerEvent, error) {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return nil, nil, err
	}
	eventID, err := platform.PathID(r, "event")
	if err != nil {
		return nil, nil, err
	}
	acct, err := h.ledgerAccountOf(r.Context(), db, id)
	if err != nil {
		return nil, nil, err
	}
	row, err := db.LedgerEvent.Query().Where(ledgerevent.IDEQ(int(eventID)), ledgerevent.WalletIDEQ(acct.WalletID)).Only(r.Context())
	if ent.IsNotFound(err) {
		return nil, nil, platform.Fail(404, "not_found")
	}
	return acct, row, err
}

func (h *handler) ledgerEventUpdate(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	db, err := h.ledgerDB()
	if err != nil {
		return err
	}
	input, err := platform.DecodeBody[LedgerEventInput](r)
	if err != nil {
		return err
	}
	if len(input.Note) > 255 || (input.Kind != "" && !ledgerManualKinds[input.Kind]) ||
		(input.AmountUSD != nil && !validLedgerAmount(input.AmountUSD)) ||
		(input.Status != "" && input.Status != "confirmed" && input.Status != "pending" && input.Status != "void") {
		return platform.Fail(400, "invalid_input")
	}
	_, row, err := h.ledgerEventOf(r, db)
	if err != nil {
		return err
	}
	loc := h.s.UsageZone()
	update := row.Update().SetUpdated(time.Now().Unix())
	if input.Kind != "" {
		update.SetKind(input.Kind)
	}
	if input.AmountUSD != nil {
		update.SetAmountUsd(*input.AmountUSD)
	}
	if input.OccurredAt != "" {
		t, ok := parseLedgerTime(input.OccurredAt, loc)
		if !ok {
			return platform.Fail(400, "invalid_input")
		}
		update.SetOccurredAt(t.Unix())
	}
	if input.Status != "" {
		update.SetStatus(input.Status)
	}
	if input.Note != "" {
		update.SetNote(input.Note)
	}
	saved, err := update.Save(r.Context())
	if err != nil {
		return err
	}
	return platform.Send(w, 200, ledgerEventView(saved, loc), nil)
}

// ledgerEventVoid 作废事件：财务事件不物理删除，只把状态改成 void。
func (h *handler) ledgerEventVoid(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	db, err := h.ledgerDB()
	if err != nil {
		return err
	}
	_, row, err := h.ledgerEventOf(r, db)
	if err != nil {
		return err
	}
	saved, err := row.Update().SetStatus("void").SetUpdated(time.Now().Unix()).Save(r.Context())
	if err != nil {
		return err
	}
	return platform.Send(w, 200, ledgerEventView(saved, h.s.UsageZone()), nil)
}

func (h *handler) ledgerAlertList(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	db, err := h.ledgerDB()
	if err != nil {
		return err
	}
	ctx, loc := r.Context(), h.s.UsageZone()
	query := db.LedgerAlert.Query()
	switch r.URL.Query().Get("status") {
	case "", "all":
	case "open":
		query = query.Where(ledgeralert.StatusEQ(ledgerAlertOpen))
	case "resolved":
		query = query.Where(ledgeralert.StatusEQ(ledgerAlertResolved))
	default:
		return platform.Fail(400, "invalid_input")
	}
	// 已忽略的告警不进入列表：契约里 status 只有 open / resolved，忽略即从告警铃移除。
	query = query.Where(ledgeralert.DismissedAtIsNil())
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return err
	}
	rows, err := query.Order(ent.Desc(ledgeralert.FieldOpenedAt), ent.Desc(ledgeralert.FieldID)).Limit(ledgerAlertListLimit).All(ctx)
	if err != nil {
		return err
	}
	names := map[int64]string{}
	accounts, err := db.LedgerAccount.Query().Where(ledgeraccount.DeletedAtIsNil()).All(ctx)
	if err != nil {
		return err
	}
	for _, acct := range accounts {
		if acct.Name != "" {
			names[acct.AccountID] = acct.Name
		}
	}
	view := LedgerAlertListView{Items: make([]LedgerAlertView, 0, len(rows)), Total: total}
	for _, row := range rows {
		accountID := row.AccountID
		name := names[accountID]
		if name == "" {
			name = fmt.Sprintf("账号 #%d", accountID)
		}
		item := LedgerAlertView{
			ID: int64(row.ID), AccountID: &accountID, AccountName: name, Kind: row.Kind, Severity: row.Severity,
			Status: row.Status, Title: ledgerAlertLabel(row.Kind, row.Value), OpenedAt: ledgerTime(row.OpenedAt, loc),
			Read: row.ReadAt != nil, Value: row.Value,
		}
		if row.ResolvedAt != nil {
			at := ledgerTime(*row.ResolvedAt, loc)
			item.ResolvedAt = &at
		}
		view.Items = append(view.Items, item)
	}
	return platform.Send(w, 200, view, nil)
}

func ledgerUnread(ctx context.Context, db *ent.Client) (int, error) {
	return db.LedgerAlert.Query().Where(ledgeralert.StatusEQ(ledgerAlertOpen), ledgeralert.ReadAtIsNil(), ledgeralert.DismissedAtIsNil()).Count(ctx)
}

func (h *handler) ledgerAlertUnread(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	db, err := h.ledgerDB()
	if err != nil {
		return err
	}
	n, err := ledgerUnread(r.Context(), db)
	if err != nil {
		return err
	}
	return platform.Send(w, 200, LedgerUnreadView{Unread: n}, nil)
}

func (h *handler) ledgerAlertReadAll(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	db, err := h.ledgerDB()
	if err != nil {
		return err
	}
	if _, err := db.LedgerAlert.Update().Where(ledgeralert.ReadAtIsNil()).SetReadAt(time.Now().Unix()).Save(r.Context()); err != nil {
		return err
	}
	return platform.Send(w, 200, LedgerUnreadView{Unread: 0}, nil)
}

func (h *handler) ledgerAlertByPath(r *http.Request, db *ent.Client) (*ent.LedgerAlert, error) {
	id, err := platform.PathID(r, "id")
	if err != nil {
		return nil, err
	}
	row, err := db.LedgerAlert.Get(r.Context(), int(id))
	if ent.IsNotFound(err) {
		return nil, platform.Fail(404, "not_found")
	}
	return row, err
}

func (h *handler) ledgerAlertRead(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	db, err := h.ledgerDB()
	if err != nil {
		return err
	}
	row, err := h.ledgerAlertByPath(r, db)
	if err != nil {
		return err
	}
	if row.ReadAt == nil {
		if err := row.Update().SetReadAt(time.Now().Unix()).Exec(r.Context()); err != nil {
			return err
		}
	}
	n, err := ledgerUnread(r.Context(), db)
	if err != nil {
		return err
	}
	return platform.Send(w, 200, LedgerUnreadView{Unread: n}, nil)
}

// ledgerAlertDismiss 忽略告警：状态改为 dismissed 并写 resolved_at，之后按 90 天保留期清理。
func (h *handler) ledgerAlertDismiss(w http.ResponseWriter, r *http.Request, a core.SessionAuth) error {
	db, err := h.ledgerDB()
	if err != nil {
		return err
	}
	row, err := h.ledgerAlertByPath(r, db)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	update := row.Update().SetStatus("dismissed").SetDismissedAt(now)
	if row.ReadAt == nil {
		update.SetReadAt(now)
	}
	if row.ResolvedAt == nil {
		update.SetResolvedAt(now)
	}
	if err := update.Exec(r.Context()); err != nil {
		return err
	}
	n, err := ledgerUnread(r.Context(), db)
	if err != nil {
		return err
	}
	return platform.Send(w, 200, LedgerUnreadView{Unread: n}, nil)
}
