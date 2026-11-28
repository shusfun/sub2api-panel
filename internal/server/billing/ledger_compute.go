package billing

import "math"

// 上游账本的纯计算函数，不访问数据库与网络，规则见 docs/decisions/0008-上游账本对账与站内告警.md。

const (
	ledgerHourSeconds       = 3600
	ledgerRechargeMinUSD    = 0.01
	ledgerSharedWalletRatio = 1.5
	ledgerSharedWalletMinUS = 0.05
)

// ledgerUsedDelta 计算两次采样间累计已用的增量。
// ok 为 false 表示任一侧缺读数；reset 为 true 表示累计值回退（基线重置），此时 delta 为 0。
func ledgerUsedDelta(prev, cur *float64) (delta float64, reset, ok bool) {
	if prev == nil || cur == nil {
		return 0, false, false
	}
	d := *cur - *prev
	if d < 0 {
		return 0, true, true
	}
	return d, false, true
}

// ledgerResetSpend 在基线重置时用“余额下降 + 期间充值”兜底估算这一段的实际花费，结果不小于 0。
func ledgerResetSpend(prevBalance, curBalance, recharged float64) float64 {
	return math.Max(0, prevBalance-curBalance+recharged)
}

// ledgerInferRecharge 推断充值：余额增加量加上钱包下各 Key 同期花费，超过阈值才算充值。
func ledgerInferRecharge(prevBalance, curBalance, keysSpend float64) (float64, bool) {
	amount := curBalance - prevBalance + keysSpend
	if amount > ledgerRechargeMinUSD {
		return amount, true
	}
	return 0, false
}

// ledgerSharedWalletSuspect 判断余额降幅是否明显大于本站可解释的花费。
func ledgerSharedWalletSuspect(balanceDrop, spend float64) bool {
	if balanceDrop <= 0 {
		return false
	}
	return balanceDrop > spend*ledgerSharedWalletRatio && balanceDrop-spend > ledgerSharedWalletMinUS
}

// ledgerHourWeight 是一段采样区间内某个小时桶的预计花费与桶起点（Unix 秒）。
type ledgerHourWeight struct {
	Bucket   int64
	Expected float64
}

// ledgerSpread 把区间 [from, to) 的实际花费摊到小时桶：
// 按各桶预计花费比例摊；预计花费合计为 0 时按各桶与区间的重叠时长摊。
func ledgerSpread(actual float64, from, to int64, hours []ledgerHourWeight) map[int64]float64 {
	out := make(map[int64]float64, len(hours))
	if actual <= 0 || to <= from || len(hours) == 0 {
		return out
	}
	var expected float64
	for _, h := range hours {
		expected += h.Expected
	}
	if expected > 0 {
		for _, h := range hours {
			out[h.Bucket] += actual * h.Expected / expected
		}
		return out
	}
	overlaps := make([]float64, len(hours))
	var total float64
	for i, h := range hours {
		lo, hi := max(h.Bucket, from), min(h.Bucket+ledgerHourSeconds, to)
		if hi > lo {
			overlaps[i] = float64(hi - lo)
			total += overlaps[i]
		}
	}
	if total == 0 {
		return out
	}
	for i, h := range hours {
		if overlaps[i] > 0 {
			out[h.Bucket] += actual * overlaps[i] / total
		}
	}
	return out
}

// ledgerDeviation 偏差 = 实际花费 − 预计花费；预计花费为 0 时不给百分比。
func ledgerDeviation(actual *float64, expected float64) (usd, pct *float64) {
	if actual == nil {
		return nil, nil
	}
	d := *actual - expected
	usd = &d
	if expected > 0 {
		p := d / expected * 100
		pct = &p
	}
	return usd, pct
}

// ledgerDeviationBreached 偏差的绝对金额和百分比同时达到阈值才告警，避免小额噪声。
func ledgerDeviationBreached(usd, pct *float64, minUSD, minPct float64) bool {
	if usd == nil || pct == nil {
		return false
	}
	return math.Abs(*usd) >= minUSD && math.Abs(*pct) >= minPct
}

// ledgerProfit 毛利 = 收入 − 花费；actual 缺失时用预计花费代替并返回 estimated=true。
func ledgerProfit(revenue float64, actual *float64, expected float64) (profit float64, estimated bool) {
	if actual == nil {
		return revenue - expected, true
	}
	return revenue - *actual, false
}

// ledgerRunwayHours 以最近每小时预计花费估算余额可撑的小时数；花费为 0 时返回 nil。
func ledgerRunwayHours(balance, hourlySpend float64) *float64 {
	if hourlySpend <= 0 || balance < 0 {
		return nil
	}
	h := balance / hourlySpend
	return &h
}

// ledgerMultiplierChanged 倍率变化超过 1e-9 视为变化；任一侧缺值不算变化。
func ledgerMultiplierChanged(prev, cur *float64) bool {
	if prev == nil || cur == nil {
		return false
	}
	return math.Abs(*prev-*cur) > 1e-9
}

// ledgerSampleUnchanged 余额、两类累计已用与倍率都没变的采样只更新 last_seen，不新增行。
func ledgerSampleUnchanged(a, b ledgerReading) bool {
	return eqFloatPtr(a.Balance, b.Balance) && eqFloatPtr(a.KeyUsed, b.KeyUsed) &&
		eqFloatPtr(a.WalletUsed, b.WalletUsed) && eqFloatPtr(a.Multiplier, b.Multiplier)
}

// ledgerReading 是一次采样的读数（美元）。
type ledgerReading struct {
	Balance    *float64
	KeyUsed    *float64
	WalletUsed *float64
	Multiplier *float64
}

func eqFloatPtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return math.Abs(*a-*b) < 1e-9
}
