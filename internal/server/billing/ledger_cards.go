package billing

import (
	"context"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/ent/ledgeraccount"
	"sub2api-panel/ent/ledgeralert"
	"sub2api-panel/ent/ledgerwallet"
	"sub2api-panel/internal/server/core"
)

// LedgerCardsFor 给账号列表取一批账号的账本摘要，由 WireLedger 注入 core.Server.LedgerCards。
// account、wallet、开启中的告警各用一次批量查询取齐；没有账本记录的账号不出现在结果里。
func LedgerCardsFor(ctx context.Context, s *core.Server, ids []int64) (map[int64]core.LedgerCardSummary, error) {
	out := map[int64]core.LedgerCardSummary{}
	if s == nil || s.Database == nil || len(ids) == 0 {
		return out, nil
	}
	db := s.Database
	accounts, err := db.LedgerAccount.Query().Where(ledgeraccount.AccountIDIn(ids...), ledgeraccount.DeletedAtIsNil()).All(ctx)
	if err != nil || len(accounts) == 0 {
		return out, err
	}
	walletIDs := make([]int, 0, len(accounts))
	seen := map[int64]bool{}
	for _, a := range accounts {
		if !seen[a.WalletID] {
			seen[a.WalletID] = true
			walletIDs = append(walletIDs, int(a.WalletID))
		}
	}
	walletRows, err := db.LedgerWallet.Query().Where(ledgerwallet.IDIn(walletIDs...)).All(ctx)
	if err != nil {
		return nil, err
	}
	wallets := make(map[int64]*ent.LedgerWallet, len(walletRows))
	for _, w := range walletRows {
		wallets[int64(w.ID)] = w
	}
	alertRows, err := db.LedgerAlert.Query().Where(ledgeralert.AccountIDIn(ids...), ledgeralert.StatusEQ(ledgerAlertOpen)).All(ctx)
	if err != nil {
		return nil, err
	}
	open := map[int64][]*ent.LedgerAlert{}
	for _, al := range alertRows {
		open[al.AccountID] = append(open[al.AccountID], al)
	}
	lc := ledgerCtx{db: db, loc: s.UsageZone(), now: time.Now(), settings: LoadLedgerSettings(ctx, db), official: s.OfficialDB != nil}
	for _, a := range accounts {
		card, err := ledgerCard(ctx, lc, a, wallets[a.WalletID], open[a.AccountID])
		if err != nil {
			return nil, err
		}
		out[a.AccountID] = card
	}
	return out, nil
}
