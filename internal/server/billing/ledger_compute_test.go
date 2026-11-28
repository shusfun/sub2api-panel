package billing

import (
	"math"
	"testing"
)

func fp(v float64) *float64 { return &v }

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestLedgerUsedDelta(t *testing.T) {
	cases := []struct {
		name      string
		prev, cur *float64
		want      float64
		reset, ok bool
	}{
		{"正常花费", fp(10), fp(12.5), 2.5, false, true},
		{"无变化", fp(10), fp(10), 0, false, true},
		{"基线重置", fp(10), fp(1), 0, true, true},
		{"缺上次读数", nil, fp(1), 0, false, false},
		{"缺本次读数", fp(1), nil, 0, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, reset, ok := ledgerUsedDelta(c.prev, c.cur)
			if !near(got, c.want) || reset != c.reset || ok != c.ok {
				t.Fatalf("got (%v,%v,%v) want (%v,%v,%v)", got, reset, ok, c.want, c.reset, c.ok)
			}
		})
	}
}

func TestLedgerResetSpend(t *testing.T) {
	cases := []struct {
		name                 string
		prev, cur, recharged float64
		want                 float64
	}{
		{"余额下降无充值", 100, 90, 0, 10},
		{"期间充值后余额上升", 100, 130, 50, 20},
		{"结果为负取零", 100, 200, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ledgerResetSpend(c.prev, c.cur, c.recharged); !near(got, c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestLedgerInferRecharge(t *testing.T) {
	cases := []struct {
		name                string
		prev, cur, keySpend float64
		want                float64
		ok                  bool
	}{
		{"充值期间扣费", 10, 60, 5, 55, true},
		{"正常消耗", 50, 45, 5, 0, false},
		{"低于阈值", 50, 50, 0.005, 0, false},
		{"恰好整数充值", 0, 100, 0, 100, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ledgerInferRecharge(c.prev, c.cur, c.keySpend)
			if ok != c.ok || (ok && !near(got, c.want)) {
				t.Fatalf("got (%v,%v) want (%v,%v)", got, ok, c.want, c.ok)
			}
		})
	}
}

func TestLedgerSharedWalletSuspect(t *testing.T) {
	cases := []struct {
		name        string
		drop, spend float64
		want        bool
	}{
		{"降幅与花费一致", 10, 10, false},
		{"降幅明显更大", 30, 10, true},
		{"余额未降", 0, 5, false},
		{"小额噪声", 0.06, 0.02, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ledgerSharedWalletSuspect(c.drop, c.spend); got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestLedgerSpread(t *testing.T) {
	t.Run("按预计花费比例跨小时摊分", func(t *testing.T) {
		got := ledgerSpread(10, 0, 7200, []ledgerHourWeight{{0, 1}, {3600, 3}})
		if !near(got[0], 2.5) || !near(got[3600], 7.5) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("预计为零时按时长摊", func(t *testing.T) {
		got := ledgerSpread(9, 1800, 3600+1800+3600, []ledgerHourWeight{{0, 0}, {3600, 0}, {7200, 0}})
		// 重叠时长 1800、3600、1800，共 7200。
		if !near(got[0], 2.25) || !near(got[3600], 4.5) || !near(got[7200], 2.25) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("单小时全部归入", func(t *testing.T) {
		got := ledgerSpread(4, 0, 600, []ledgerHourWeight{{0, 2}})
		if !near(got[0], 4) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("零花费与空区间不产出", func(t *testing.T) {
		if len(ledgerSpread(0, 0, 3600, []ledgerHourWeight{{0, 1}})) != 0 ||
			len(ledgerSpread(5, 100, 100, []ledgerHourWeight{{0, 1}})) != 0 {
			t.Fatal("expected empty")
		}
	})
	t.Run("摊分总和等于实际花费", func(t *testing.T) {
		got := ledgerSpread(7.77, 100, 9000, []ledgerHourWeight{{0, 1.3}, {3600, 0.4}, {7200, 2.2}})
		var sum float64
		for _, v := range got {
			sum += v
		}
		if !near(sum, 7.77) {
			t.Fatalf("sum %v", sum)
		}
	})
}

func TestLedgerDeviation(t *testing.T) {
	usd, pct := ledgerDeviation(fp(12), 10)
	if usd == nil || pct == nil || !near(*usd, 2) || !near(*pct, 20) {
		t.Fatalf("got %v %v", usd, pct)
	}
	if usd, pct := ledgerDeviation(nil, 10); usd != nil || pct != nil {
		t.Fatal("缺实际花费不应有偏差")
	}
	if usd, pct := ledgerDeviation(fp(3), 0); usd == nil || pct != nil {
		t.Fatal("预计为零时只给金额偏差")
	}
}

func TestLedgerDeviationBreached(t *testing.T) {
	cases := []struct {
		name     string
		usd, pct *float64
		want     bool
	}{
		{"两项都达到", fp(2), fp(20), true},
		{"负偏差同样触发", fp(-2), fp(-20), true},
		{"金额不足", fp(0.5), fp(50), false},
		{"比例不足", fp(5), fp(5), false},
		{"缺比例", fp(5), nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ledgerDeviationBreached(c.usd, c.pct, 1, 10); got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestLedgerProfit(t *testing.T) {
	if p, est := ledgerProfit(10, fp(6), 5); !near(p, 4) || est {
		t.Fatalf("got %v %v", p, est)
	}
	if p, est := ledgerProfit(10, nil, 5); !near(p, 5) || !est {
		t.Fatalf("got %v %v", p, est)
	}
}

func TestLedgerRunwayHours(t *testing.T) {
	if h := ledgerRunwayHours(60, 10); h == nil || !near(*h, 6) {
		t.Fatalf("got %v", h)
	}
	if ledgerRunwayHours(60, 0) != nil || ledgerRunwayHours(-1, 5) != nil {
		t.Fatal("零花费或负余额不应给出可撑时长")
	}
}

func TestLedgerMultiplierChanged(t *testing.T) {
	if !ledgerMultiplierChanged(fp(1), fp(1.2)) || ledgerMultiplierChanged(fp(1), fp(1)) ||
		ledgerMultiplierChanged(nil, fp(1)) || ledgerMultiplierChanged(fp(1), nil) {
		t.Fatal("倍率变化判断错误")
	}
}

func TestLedgerSampleUnchanged(t *testing.T) {
	base := ledgerReading{Balance: fp(10), KeyUsed: fp(2), WalletUsed: nil, Multiplier: fp(1)}
	same := ledgerReading{Balance: fp(10), KeyUsed: fp(2), WalletUsed: nil, Multiplier: fp(1)}
	if !ledgerSampleUnchanged(base, same) {
		t.Fatal("读数相同应视为未变化")
	}
	changed := same
	changed.Balance = fp(9)
	if ledgerSampleUnchanged(base, changed) {
		t.Fatal("余额变化应视为已变化")
	}
	missing := same
	missing.Multiplier = nil
	if ledgerSampleUnchanged(base, missing) {
		t.Fatal("一侧缺值应视为已变化")
	}
}
