package pool

import "context"

// withLedgerExtras 给账号列表合并近 5 分钟 RPM、平均延迟与上游账本摘要。
// RPM 与延迟只在接了官方只读库时出现；账本摘要由 billing 经 Server.LedgerCards 注入，
// 未注入或查询失败时跳过 ledger 字段，不影响列表。
func (h *handler) withLedgerExtras(ctx context.Context, items []AccountAdminView, ids []int64) {
	if len(items) == 0 {
		return
	}
	if h.s.OfficialDB != nil {
		if stats, err := h.accountRecentStats(ctx, ids); err == nil {
			for i := range items {
				stat := stats[items[i].ID]
				rpm := stat.RPM5m
				items[i].RPM5m = &rpm
				items[i].AvgLatencyMs = stat.AvgLatencyMS
			}
		}
	}
	if h.s.LedgerCards == nil {
		return
	}
	cards, err := h.s.LedgerCards(ctx, ids)
	if err != nil {
		return
	}
	for i := range items {
		if card, ok := cards[items[i].ID]; ok {
			c := card
			items[i].Ledger = &c
		}
	}
}
