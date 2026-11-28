package billing

import (
	"context"
	"fmt"
	"time"

	"sub2api-panel/internal/platform"
)

// 未接官方只读库时的按天退回：用当前管理员会话请求上游 GET /admin/accounts/:id/stats?days=N，
// 取 history[] 的 date、requests、actual_cost（账号口径，即预计花费）与 user_cost（用户口径，即本站收入）。
// 上游只给总额，不区分余额请求与订阅消耗，所以全部记在余额请求成本里；也没有上游真实扣费，actual 留空。
// 后台汇总没有管理员令牌（管理密钥只在 issuer 进程），所以只能在请求时用会话令牌补，不落库。
func (h *handler) ledgerDailyFallback(ctx context.Context, token string, id int64, start, end time.Time, loc *time.Location, showProfit bool) []LedgerPointView {
	days := int(end.Sub(start).Hours()/24) + 1
	days = min(max(days, 1), 90)
	wire, err := platform.Upstream[struct {
		History []struct {
			Date       string  `json:"date"`
			Requests   int64   `json:"requests"`
			ActualCost float64 `json:"actual_cost"`
			UserCost   float64 `json:"user_cost"`
		} `json:"history"`
	}](ctx, h.s.Upstream, "GET", fmt.Sprintf("/admin/accounts/%d/stats?days=%d", id, days), token, "", "", nil)
	if err != nil {
		return nil
	}
	out := make([]LedgerPointView, 0, len(wire.History))
	for _, row := range wire.History {
		day, err := time.ParseInLocation("2006-01-02", row.Date, loc)
		if err != nil || day.Before(ledgerDayStart(start, loc)) || day.After(end) {
			continue
		}
		point := LedgerPointView{Bucket: ledgerTime(day.Unix(), loc), Requests: row.Requests, ExpectedBalanceCostUSD: row.ActualCost}
		if showProfit {
			revenue := row.UserCost
			point.RevenueUSD = &revenue
		}
		out = append(out, point)
	}
	return out
}
