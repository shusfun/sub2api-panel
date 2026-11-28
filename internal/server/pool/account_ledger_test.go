package pool

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"sub2api-panel/internal/server/core"
)

const ledgerLeak = "fixture-not-for-client"

func ledgerTestItems() ([]AccountAdminView, []int64) {
	return []AccountAdminView{{ID: 5}, {ID: 6}}, []int64{5, 6}
}

func TestWithLedgerExtrasMergesCards(t *testing.T) {
	provider := "newapi"
	h := &handler{s: &core.Server{LedgerCards: func(_ context.Context, ids []int64) (map[int64]core.LedgerCardSummary, error) {
		if len(ids) != 2 {
			t.Fatalf("should fetch the whole batch at once: %v", ids)
		}
		return map[int64]core.LedgerCardSummary{5: {Provider: &provider, Currency: "USD", Granularity: "hour", Alerts: []core.LedgerAlertBadge{}}}, nil
	}}}
	items, ids := ledgerTestItems()
	h.withLedgerExtras(context.Background(), items, ids)
	if items[0].Ledger == nil || items[1].Ledger != nil {
		t.Fatalf("only accounts with ledger rows carry ledger: %+v %+v", items[0].Ledger, items[1].Ledger)
	}
	if items[0].RPM5m != nil || items[0].AvgLatencyMs != nil {
		t.Fatal("rpm_5m and avg_latency_ms must be absent without official db")
	}
	raw, _ := json.Marshal(items[0])
	if strings.Contains(string(raw), ledgerLeak) || strings.Contains(string(raw), "rpm_5m") || strings.Contains(string(raw), "avg_latency_ms") {
		t.Fatalf("unexpected fields: %s", raw)
	}
	var wire struct {
		Ledger map[string]any `json:"ledger"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"provider": true, "currency": true, "reconcile": true, "granularity": true, "balance_usd": true, "runway_hours": true,
		"multiplier": true, "local_multiplier": true, "previous_multiplier": true, "actual_today_usd": true, "expected_today_usd": true,
		"deviation_usd": true, "deviation_pct": true, "revenue_today_usd": true, "profit_today_usd": true, "estimated": true,
		"sampled_at": true, "stale": true, "alerts": true}
	for key := range wire.Ledger {
		if !allowed[key] {
			t.Fatalf("ledger has non-whitelisted field %q", key)
		}
	}
}

func TestWithLedgerExtrasSkipsWhenUnavailable(t *testing.T) {
	cases := map[string]*core.Server{
		"not wired": {},
		"query failed": {LedgerCards: func(context.Context, []int64) (map[int64]core.LedgerCardSummary, error) {
			return nil, errors.New("db down")
		}},
	}
	for name, server := range cases {
		t.Run(name, func(t *testing.T) {
			items, ids := ledgerTestItems()
			(&handler{s: server}).withLedgerExtras(context.Background(), items, ids)
			for _, item := range items {
				if item.Ledger != nil || item.RPM5m != nil {
					t.Fatalf("must not carry ledger fields: %+v", item)
				}
			}
		})
	}
}
