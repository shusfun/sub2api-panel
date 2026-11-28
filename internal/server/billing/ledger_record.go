package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"time"

	"sub2api-panel/ent"
	"sub2api-panel/ent/ledgeraccount"
	"sub2api-panel/ent/ledgersample"
	"sub2api-panel/ent/ledgerwallet"
	"sub2api-panel/internal/server/core"
)

// 采样落库、钱包归属与事件写入。规则见 docs/decisions/0008-上游账本对账与站内告警.md。

const (
	ledgerKindKey     = "key"
	ledgerKindWallet  = "wallet"
	ledgerKindFailure = "failure"
)

// ledgerFingerprint 用面板密钥对任意标识做 HMAC，库里只存指纹，不存 Key 或令牌明文。
func ledgerFingerprint(secret string, parts ...string) string {
	mac := hmac.New(sha256.New, []byte("panel ledger\x00"+secret))
	mac.Write([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(mac.Sum(nil))
}

// ledgerWalletIdentity 返回钱包指纹与站点来源：
// 有访问令牌且知道上游用户 ID 时按“站点 + 用户 ID”归属，同一用户下的多把 Key 共用一个钱包；
// 否则按 Key 指纹归属，一把 Key 一个钱包。
func ledgerWalletIdentity(secret string, config core.BalanceConfig) (identity, origin, keyFingerprint string) {
	if u, err := url.Parse(strings.TrimSpace(config.BaseURL)); err == nil {
		origin = strings.ToLower(u.Hostname())
	}
	if origin == "" {
		origin = config.Provider
	}
	if config.Key != "" {
		keyFingerprint = ledgerFingerprint(secret, "key", origin, config.Key)
	}
	if config.AccessToken != "" && config.UpstreamUserID > 0 {
		return ledgerFingerprint(secret, "user", origin, strconv.FormatInt(config.UpstreamUserID, 10)), origin, keyFingerprint
	}
	if keyFingerprint != "" {
		return keyFingerprint, origin, keyFingerprint
	}
	return ledgerFingerprint(secret, "account", config.Provider, origin), origin, ""
}

func ledgerEnsureWallet(ctx context.Context, db *ent.Client, identity, provider, origin string, perUSD float64, upstreamUserID int64) (*ent.LedgerWallet, error) {
	now := time.Now().Unix()
	find := func() (*ent.LedgerWallet, error) {
		return db.LedgerWallet.Query().Where(ledgerwallet.IdentityEQ(identity)).Only(ctx)
	}
	row, err := find()
	if ent.IsNotFound(err) {
		row, err = db.LedgerWallet.Create().SetIdentity(identity).SetProvider(provider).SetOrigin(origin).
			SetQuotaPerUsd(perUSD).SetUpstreamUserID(upstreamUserID).SetCreated(now).SetUpdated(now).Save(ctx)
		if ent.IsConstraintError(err) {
			return find()
		}
		return row, err
	}
	if err != nil {
		return nil, err
	}
	if (perUSD > 0 && row.QuotaPerUsd != perUSD) || (upstreamUserID > 0 && row.UpstreamUserID != upstreamUserID) {
		update := row.Update().SetUpdated(now)
		if perUSD > 0 {
			update.SetQuotaPerUsd(perUSD)
		}
		if upstreamUserID > 0 {
			update.SetUpstreamUserID(upstreamUserID)
		}
		return update.Save(ctx)
	}
	return row, nil
}

// ledgerEnsureAccount 建立或更新账号的钱包归属；账号重新出现时清除删除标记。
func ledgerEnsureAccount(ctx context.Context, db *ent.Client, accountID, walletID int64, keyFingerprint string) error {
	now := time.Now().Unix()
	row, err := db.LedgerAccount.Query().Where(ledgeraccount.AccountIDEQ(accountID)).Only(ctx)
	if ent.IsNotFound(err) {
		_, err = db.LedgerAccount.Create().SetAccountID(accountID).SetWalletID(walletID).
			SetKeyFingerprint(keyFingerprint).SetCreated(now).SetUpdated(now).Save(ctx)
		if ent.IsConstraintError(err) {
			return nil
		}
		return err
	}
	if err != nil {
		return err
	}
	if row.WalletID == walletID && row.KeyFingerprint == keyFingerprint && row.DeletedAt == nil {
		return nil
	}
	_, err = row.Update().SetWalletID(walletID).SetKeyFingerprint(keyFingerprint).ClearDeletedAt().SetUpdated(now).Save(ctx)
	return err
}

// ledgerReadingOf 把一次上游读数折成账本读数。非美元币种（如 kimi 的 CNY）不参与对账，余额置空。
func ledgerReadingOf(view core.AccountBalanceView) ledgerReading {
	var r ledgerReading
	if view.Currency == "USD" {
		balance := view.Balance
		r.Balance = &balance
	}
	if view.Sample != nil {
		r.KeyUsed, r.WalletUsed = view.Sample.KeyUsedUSD, view.Sample.WalletUsedUSD
	}
	if view.Billing != nil {
		m := view.Billing.Effective
		r.Multiplier = &m
	}
	return r
}

func ledgerReadingOfRow(row *ent.LedgerSample) ledgerReading {
	return ledgerReading{Balance: row.BalanceUsd, KeyUsed: row.KeyUsedUsd, WalletUsed: row.WalletUsedUsd, Multiplier: row.Multiplier}
}

func ledgerSampleKind(config core.BalanceConfig) string {
	if config.AccessToken != "" {
		return ledgerKindWallet
	}
	return ledgerKindKey
}

// RecordLedgerSample 是采样成功回调：确保钱包与账号归属，读数没变只刷新 last_seen，变了才新增一行。
func RecordLedgerSample(ctx context.Context, s *core.Server, accountID int64, config core.BalanceConfig, view core.AccountBalanceView) error {
	if s == nil || s.Database == nil {
		return nil
	}
	identity, origin, keyFingerprint := ledgerWalletIdentity(s.Secret, config)
	var perUSD float64
	if view.Sample != nil {
		perUSD = view.Sample.QuotaPerUSD
	}
	wallet, err := ledgerEnsureWallet(ctx, s.Database, identity, config.Provider, origin, perUSD, config.UpstreamUserID)
	if err != nil {
		return err
	}
	if err := ledgerEnsureAccount(ctx, s.Database, accountID, int64(wallet.ID), keyFingerprint); err != nil {
		return err
	}
	now := time.Now().Unix()
	kind, reading := ledgerSampleKind(config), ledgerReadingOf(view)
	last, err := s.Database.LedgerSample.Query().Where(ledgersample.AccountIDEQ(accountID)).
		Order(ent.Desc(ledgersample.FieldLastSeen)).First(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return err
	}
	if err == nil && last.Kind == kind && last.WalletID == int64(wallet.ID) && ledgerSampleUnchanged(ledgerReadingOfRow(last), reading) {
		return last.Update().SetLastSeen(now).Exec(ctx)
	}
	return s.Database.LedgerSample.Create().SetWalletID(int64(wallet.ID)).SetAccountID(accountID).
		SetFirstSeen(now).SetLastSeen(now).SetNillableBalanceUsd(reading.Balance).
		SetNillableKeyUsedUsd(reading.KeyUsed).SetNillableWalletUsedUsd(reading.WalletUsed).
		SetNillableMultiplier(reading.Multiplier).SetKind(kind).SetCurrency(ledgerCurrency(view)).Exec(ctx)
}

func ledgerCurrency(view core.AccountBalanceView) string {
	if view.Currency == "" {
		return "USD"
	}
	return view.Currency
}

// RecordLedgerFailure 是采样失败回调：写一条 failure 样本，连续失败次数由告警求值按最近样本统计。
// 连续失败期间同一钱包只保留一行，只刷新 last_seen。
func RecordLedgerFailure(ctx context.Context, s *core.Server, accountID int64, config core.BalanceConfig) error {
	if s == nil || s.Database == nil {
		return nil
	}
	identity, origin, keyFingerprint := ledgerWalletIdentity(s.Secret, config)
	wallet, err := ledgerEnsureWallet(ctx, s.Database, identity, config.Provider, origin, 0, config.UpstreamUserID)
	if err != nil {
		return err
	}
	if err := ledgerEnsureAccount(ctx, s.Database, accountID, int64(wallet.ID), keyFingerprint); err != nil {
		return err
	}
	now := time.Now().Unix()
	return s.Database.LedgerSample.Create().SetWalletID(int64(wallet.ID)).SetAccountID(accountID).
		SetFirstSeen(now).SetLastSeen(now).SetKind(ledgerKindFailure).SetCurrency("USD").Exec(ctx)
}
