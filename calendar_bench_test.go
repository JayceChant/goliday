// 黑盒基准测试（package goliday_test）：覆盖查询热路径（单日判定、
// 区间统计、离散统计），为前缀和/稀疏终态表等性能取向的实现提供
// 回归基线；数据加载一次复用（构造完成后只读、并发安全）。
package goliday_test

import (
	"sync"
	"testing"
	"time"

	"github.com/JayceChant/goliday"
)

// benchCalendar 进程内仅加载一次 testdata 真实配置（2025、2026）。
var benchCalendar = sync.OnceValue(func() *goliday.Calendar {
	store, err := goliday.LoadDir("testdata")
	if err != nil {
		panic("基准加载测试配置失败: " + err.Error())
	}
	return goliday.NewCalendar(store)
})

// benchDates 构造 [2026-01-01, 2026-12-31] 全年日期（含边界调休/补班日）。
func benchDates() []time.Time {
	days := make([]time.Time, 0, 365)
	for d := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC); d.Year() == 2026; d = d.AddDate(0, 0, 1) {
		days = append(days, d)
	}
	return days
}

// BenchmarkQuery 单日细粒度判定（稀疏终态表命中 + 周休回退混合路径）。
func BenchmarkQuery(b *testing.B) {
	c := benchCalendar()
	dates := benchDates()
	b.ReportAllocs()
	b.ResetTimer()
	i := 0
	for b.Loop() {
		if _, err := c.Query(dates[i%len(dates)]); err != nil {
			b.Fatalf("Query 意外报错: %v", err)
		}
		i++
	}
}

// BenchmarkQueryCoarse 单日粗粒度判定（Query + 投影）。
func BenchmarkQueryCoarse(b *testing.B) {
	c := benchCalendar()
	dates := benchDates()
	b.ReportAllocs()
	b.ResetTimer()
	i := 0
	for b.Loop() {
		if _, err := c.QueryCoarse(dates[i%len(dates)]); err != nil {
			b.Fatalf("QueryCoarse 意外报错: %v", err)
		}
		i++
	}
}

// BenchmarkStatsRangeFullYear 全年区间统计（单年前缀和差分 + 结果导出）。
func BenchmarkStatsRangeFullYear(b *testing.B) {
	c := benchCalendar()
	s := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := c.StatsRange(s, e, true); err != nil {
			b.Fatalf("StatsRange 意外报错: %v", err)
		}
	}
}

// BenchmarkStatsList 离散列表统计（内部排序去重 + 逐日单日差分）。
func BenchmarkStatsList(b *testing.B) {
	c := benchCalendar()
	dates := benchDates()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := c.Stats(dates, true); err != nil {
			b.Fatalf("Stats 意外报错: %v", err)
		}
	}
}

// BenchmarkQueryRangeFullYear 全年区间逐日明细（365 个 Dated 分配）。
func BenchmarkQueryRangeFullYear(b *testing.B) {
	c := benchCalendar()
	s := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := c.QueryRange(s, e); err != nil {
			b.Fatalf("QueryRange 意外报错: %v", err)
		}
	}
}
