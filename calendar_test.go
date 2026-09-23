// 黑盒测试（package goliday_test）：仅经导出 API 验证 Calendar 判定、
// 区间与统计的对外契约（判断算法、粗细映射、未加载年份报错、前缀和统计）。
package goliday_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/JayceChant/goliday"
)

// newTestCalendar 加载 testdata 真实配置（2025、2026）构造 Calendar。
func newTestCalendar(t *testing.T) *goliday.Calendar {
	t.Helper()
	return goliday.NewCalendar(mustLoadDir(t, "testdata"))
}

// wantDayType 断言 Query/QueryCoarse 返回值与错误。
func wantDayType(t *testing.T, c *goliday.Calendar, name string, d time.Time, want, coarse goliday.DayType) {
	t.Helper()
	got, err := c.Query(d)
	if err != nil {
		t.Fatalf("%s: Query 意外报错: %v", name, err)
	}
	if got != want {
		t.Errorf("%s: Query = %d（%s），期望 %d（%s）", name, got, got, want, want)
	}
	gotC, err := c.QueryCoarse(d)
	if err != nil {
		t.Fatalf("%s: QueryCoarse 意外报错: %v", name, err)
	}
	if gotC != coarse {
		t.Errorf("%s: QueryCoarse = %d（%s），期望 %d（%s）", name, gotC, gotC, coarse, coarse)
	}
}

// TestCalendarQuery 单日细/粗粒度查询断言（2026 假设方案、2025 真实数据）。
func TestCalendarQuery(t *testing.T) {
	c := newTestCalendar(t)

	tests := []struct {
		name   string
		date   string
		want   goliday.DayType
		coarse goliday.DayType
	}{
		{"2026-02-20 周五调休off", "2026-02-20", goliday.DayTypeAdjustedRestDay, goliday.DayTypeRest},
		{"2026-02-17 春节当天且off", "2026-02-17", goliday.DayTypeFestivalRest, goliday.DayTypeRest},
		{"2026-02-28 周六补班work", "2026-02-28", goliday.DayTypeAdjustedWorkDay, goliday.DayTypeWork},
		{"2026-04-05 周日清明当天不在off/work", "2026-04-05", goliday.DayTypeFestivalRest, goliday.DayTypeRest},
		{"2026-03-03 周二未覆盖", "2026-03-03", goliday.DayTypeWork, goliday.DayTypeWork},
		{"2026-02-21 周六春节假期内自然周末", "2026-02-21", goliday.DayTypeRest, goliday.DayTypeRest},
		{"2025-01-26 周日补班", "2025-01-26", goliday.DayTypeAdjustedWorkDay, goliday.DayTypeWork},
		{"2025-01-29 正月初一", "2025-01-29", goliday.DayTypeFestivalRest, goliday.DayTypeRest},
		{"2025-10-06 中秋当天周一", "2025-10-06", goliday.DayTypeFestivalRest, goliday.DayTypeRest},
		{"2025-04-05 周六假期内自然周末", "2025-04-05", goliday.DayTypeRest, goliday.DayTypeRest},
	}
	for _, tt := range tests {
		wantDayType(t, c, tt.name, date(t, tt.date), tt.want, tt.coarse)
	}
}

// TestCalendarQueryNormalization 同一日不同时刻/时区的输入应得到一致结果。
func TestCalendarQueryNormalization(t *testing.T) {
	c := newTestCalendar(t)

	want := goliday.DayTypeFestivalRest
	variants := []time.Time{
		date(t, "2026-02-17"),
		date(t, "2026-02-17").Add(15*time.Hour + 30*time.Minute),
		time.Date(2026, 2, 17, 23, 59, 59, 0, time.FixedZone("CST", 8*3600)),
	}
	for i, v := range variants {
		got, err := c.Query(v)
		if err != nil {
			t.Fatalf("variants[%d] Query 意外报错: %v", i, err)
		}
		if got != want {
			t.Errorf("variants[%d]（%s）Query = %d（%s），期望 %d（%s）",
				i, v.Format("2006-01-02 15:04:05 MST"), got, got, want, want)
		}
	}
}

// TestCalendarYearNotLoaded 未加载年份：Query/QueryCoarse/IsWork/
// IsRest/QueryRange/StatsRange/Stats 均返回包装 ErrYearNotLoaded 的
// 错误（不再静默回退周休判断），错误消息包含年份。
func TestCalendarYearNotLoaded(t *testing.T) {
	c := newTestCalendar(t)

	d := date(t, "2027-05-01")
	for name, fn := range map[string]func() error{
		"Query":       func() error { _, err := c.Query(d); return err },
		"QueryCoarse": func() error { _, err := c.QueryCoarse(d); return err },
		"IsWork":      func() error { _, err := c.IsWork(d); return err },
		"IsRest":      func() error { _, err := c.IsRest(d); return err },
	} {
		err := fn()
		if !errors.Is(err, goliday.ErrYearNotLoaded) {
			t.Errorf("%s(2027-05-01) 错误 = %v，期望包装 ErrYearNotLoaded", name, err)
			continue
		}
		if got := err.Error(); !stringsContains(got, "2027") {
			t.Errorf("%s 错误消息 %q 应包含年份 2027", name, got)
		}
	}

	if _, err := c.QueryRange(date(t, "2027-01-01"), date(t, "2027-01-05")); !errors.Is(err, goliday.ErrYearNotLoaded) {
		t.Errorf("QueryRange(2027) 错误 = %v，期望包装 ErrYearNotLoaded", err)
	}
	if _, err := c.QueryRange(date(t, "2026-12-30"), date(t, "2027-01-05")); !errors.Is(err, goliday.ErrYearNotLoaded) {
		t.Errorf("QueryRange 跨入 2027 错误 = %v，期望包装 ErrYearNotLoaded", err)
	}
	if _, err := c.StatsRange(date(t, "2025-06-01"), date(t, "2027-12-31"), true); !errors.Is(err, goliday.ErrYearNotLoaded) {
		t.Errorf("StatsRange 含未加载中间年 2026 之外场景错误 = %v，期望包装 ErrYearNotLoaded", err)
	}
	if _, err := c.Stats([]time.Time{date(t, "2025-01-02"), date(t, "2028-03-01")}, true); !errors.Is(err, goliday.ErrYearNotLoaded) {
		t.Errorf("Stats 含未加载年错误 = %v，期望包装 ErrYearNotLoaded", err)
	}

	// 空区间无覆盖年份，不校验且返回全零。
	r, err := c.StatsRange(date(t, "2027-01-01"), date(t, "2027-01-01"), true)
	if err != nil {
		t.Fatalf("空区间 StatsRange 意外报错: %v", err)
	}
	if r.Total != 0 || len(r.Coarse) != 0 || len(r.Fine) != 0 {
		t.Errorf("空区间 StatsRange = %+v，期望全零", r)
	}
	ds, err := c.QueryRange(date(t, "2027-01-01"), date(t, "2027-01-01"))
	if err != nil || len(ds) != 0 {
		t.Errorf("空区间 QueryRange = %v, %v，期望空且无错", ds, err)
	}
}

// stringsContains 简易包含判断（避免引入额外依赖分支）。
func stringsContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestCalendarQueryRange 区间查询：左闭右开逐日、end<=start 返回空切片。
func TestCalendarQueryRange(t *testing.T) {
	c := newTestCalendar(t)

	got, err := c.QueryRange(date(t, "2026-02-14"), date(t, "2026-02-17"))
	if err != nil {
		t.Fatalf("QueryRange 意外报错: %v", err)
	}
	want := []goliday.Dated{
		{date(t, "2026-02-14"), goliday.DayTypeAdjustedWorkDay},
		{date(t, "2026-02-15"), goliday.DayTypeRest},
		{date(t, "2026-02-16"), goliday.DayTypeAdjustedRestDay},
	}
	if len(got) != len(want) {
		t.Fatalf("QueryRange 返回 %d 个元素，期望 %d（左闭右开不含 end）：%v", len(got), len(want), got)
	}
	for i, w := range want {
		if !got[i].Date.Equal(w.Date) || got[i].Type != w.Type {
			t.Errorf("days[%d] = {%s %d（%s）}，期望 {%s %d（%s）}",
				i, got[i].Date.Format(layout), got[i].Type, got[i].Type,
				w.Date.Format(layout), w.Type, w.Type)
		}
	}

	if d, err := c.QueryRange(date(t, "2026-02-14"), date(t, "2026-02-14")); err != nil || len(d) != 0 {
		t.Errorf("start == end 应返回空切片且无错，got %v, %v", d, err)
	}
	if d, err := c.QueryRange(date(t, "2026-02-17"), date(t, "2026-02-14")); err != nil || len(d) != 0 {
		t.Errorf("end < start 应返回空切片且无错，got %v, %v", d, err)
	}
}

// fineKeyOf 将细粒度值映射为 Fine 计数键：键为五个合法终态组合值
// （1/2/6/10/17），组合值即键本身，普通工作日/周休以基本位单值为键。
func fineKeyOf(dt goliday.DayType) goliday.DayType {
	return dt // Work(1)/Rest(2) 与三个组合值 6/10/17 本身即 Fine 键
}

// bruteForceStats 逐日 Query 暴力统计，作为前缀和路径的一致性基准。
// 细粒度为按值 MECE 计数（键为五个终态组合值 {1,2,6,10,17}），
// 粗粒度为基本位投影。
func bruteForceStats(t *testing.T, c *goliday.Calendar, start, end time.Time, detailed bool) goliday.StatsResult {
	t.Helper()
	r := goliday.StatsResult{Total: 0, Coarse: map[goliday.DayType]int{}}
	if detailed {
		r.Fine = map[goliday.DayType]int{}
	}
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		dt, err := c.Query(d)
		if err != nil {
			t.Fatalf("暴力统计 Query(%s) 意外报错: %v", d.Format(layout), err)
		}
		r.Total++
		r.Coarse[dt.Coarse()]++
		if detailed {
			r.Fine[fineKeyOf(dt)]++
		}
	}
	return r
}

// wantSameStats 断言前缀和结果与暴力基准完全一致。
func wantSameStats(t *testing.T, name string, got, want goliday.StatsResult) {
	t.Helper()
	if got.Total != want.Total {
		t.Errorf("%s: Total = %d，期望 %d", name, got.Total, want.Total)
	}
	if got.Coarse[goliday.DayTypeWork] != want.Coarse[goliday.DayTypeWork] ||
		got.Coarse[goliday.DayTypeRest] != want.Coarse[goliday.DayTypeRest] {
		t.Errorf("%s: Coarse = %v，期望 %v", name, got.Coarse, want.Coarse)
	}
	if len(want.Fine) > 0 {
		fineKeys := []goliday.DayType{
			goliday.DayTypeWork, goliday.DayTypeRest, goliday.DayTypeFestivalRest,
			goliday.DayTypeAdjustedRestDay, goliday.DayTypeAdjustedWorkDay,
		}
		for _, k := range fineKeys {
			if got.Fine[k] != want.Fine[k] {
				t.Errorf("%s: Fine[%s] = %d，期望 %d", name, k, got.Fine[k], want.Fine[k])
			}
		}
	}
}

// TestCalendarStats 统计：细粒度五键 MECE 与粗粒度模式断言（前缀和路径）。
func TestCalendarStats(t *testing.T) {
	c := newTestCalendar(t)
	dates := []time.Time{date(t, "2026-02-17"), date(t, "2026-02-28"), date(t, "2026-03-03")}

	r, err := c.Stats(dates, true)
	if err != nil {
		t.Fatalf("Stats 意外报错: %v", err)
	}
	if r.Total != 3 {
		t.Errorf("Total = %d，期望 3", r.Total)
	}
	if r.Coarse[goliday.DayTypeWork] != 2 || r.Coarse[goliday.DayTypeRest] != 1 {
		t.Errorf("Coarse = %v，期望 work=2、holiday=1", r.Coarse)
	}
	// 02-17 节日放假日、02-28 补班日、03-03 普通工作日：三键各计 1，
	// 五键之和 == Total（MECE）。
	if len(r.Fine) != 5 {
		t.Fatalf("Fine = %v，期望 5 键（MECE）", r.Fine)
	}
	wantFine := map[goliday.DayType]int{
		goliday.DayTypeFestivalRest:    1,
		goliday.DayTypeAdjustedWorkDay: 1,
		goliday.DayTypeWork:            1,
		goliday.DayTypeRest:            0,
		goliday.DayTypeAdjustedRestDay: 0,
	}
	sum := 0
	for k, v := range r.Fine {
		if wantFine[k] != v {
			t.Errorf("Fine[%d（%s）] = %d，期望 %d", k, k, v, wantFine[k])
		}
		sum += v
	}
	if sum != r.Total {
		t.Errorf("Fine 五键之和 = %d，期望 == Total = %d", sum, r.Total)
	}

	rc, err := c.Stats(dates, false)
	if err != nil {
		t.Fatalf("Stats（粗粒度）意外报错: %v", err)
	}
	if rc.Total != 3 {
		t.Errorf("粗粒度模式 Total = %d，期望 3", rc.Total)
	}
	if rc.Coarse[goliday.DayTypeWork] != 2 || rc.Coarse[goliday.DayTypeRest] != 1 {
		t.Errorf("粗粒度模式 Coarse = %v，期望 work=2、holiday=1", rc.Coarse)
	}
	if len(rc.Fine) != 0 {
		t.Errorf("粗粒度模式 Fine 应无计数，got %v", rc.Fine)
	}

	if r, err := c.Stats(nil, true); err != nil || r.Total != 0 || len(r.Coarse) != 0 || len(r.Fine) != 0 {
		t.Errorf("空输入 Stats = %+v, %v，期望全零且无错", r, err)
	}
}

// TestStatsRangeNoAdjustmentOverflow 无调整年（空 off/work、无节日，校验
// 允许的合法配置）的普通工作日可达 262 天（闰年且元旦为周一，2024 即是），
// 超出 uint8 上界 255：前缀和元素曾以 uint8 存储，累计到 256 即回绕，
// 导致五键之和 ≠ total_days（MECE 不变量破坏）。本回归固化 uint16 存储
// 的正确性。
func TestStatsRangeNoAdjustmentOverflow(t *testing.T) {
	dir := t.TempDir()
	// 2024：闰年 366 天、元旦为周一（周末恰 104 天），无任何调整时
	// ordinary = 262 > 255。
	writeYearFile(t, dir, "2024.toml", "year = 2024\n\n[adjust]\noff = []\nwork = []\n")
	c := goliday.NewCalendar(mustLoadDir(t, dir))

	r, err := c.StatsRange(date(t, "2024-01-01"), date(t, "2025-01-01"), true)
	if err != nil {
		t.Fatalf("StatsRange 意外报错: %v", err)
	}
	if r.Total != 366 {
		t.Errorf("Total = %d，期望 366（闰年全年）", r.Total)
	}
	if got := r.Fine[goliday.DayTypeWork]; got != 262 {
		t.Errorf("Fine[ordinary] = %d，期望 262（uint8 回绕会得到 6）", got)
	}
	if got := r.Fine[goliday.DayTypeRest]; got != 104 {
		t.Errorf("Fine[weekend] = %d，期望 104", got)
	}
	sum := 0
	for _, v := range r.Fine {
		sum += v
	}
	if sum != r.Total {
		t.Errorf("五键之和 = %d，期望 == Total = %d（MECE）", sum, r.Total)
	}
}

// TestStatsDefensiveNormalization Stats 对输入做内部排序去重：乱序、重复
// 输入与排序去重输入结果一致；入参切片不被修改；首个日期为 0 年时不再
// 跳过年份校验（原 prev := 0 哨兵会漏判导致空指针 panic，应返回错误）。
func TestStatsDefensiveNormalization(t *testing.T) {
	c := newTestCalendar(t)

	sorted := []time.Time{date(t, "2025-12-31"), date(t, "2026-01-01"), date(t, "2026-02-17")}
	messy := []time.Time{
		date(t, "2026-02-17"), date(t, "2025-12-31"), date(t, "2026-01-01"),
		date(t, "2026-02-17"), date(t, "2025-12-31"),
	}
	want, err := c.Stats(sorted, true)
	if err != nil {
		t.Fatalf("Stats（有序输入）意外报错: %v", err)
	}
	backup := slicesClone(messy)
	got, err := c.Stats(messy, true)
	if err != nil {
		t.Fatalf("Stats（乱序重复输入）意外报错: %v", err)
	}
	if got.Total != want.Total || len(got.Fine) != len(want.Fine) {
		t.Fatalf("乱序重复输入 Total = %d，期望与去重输入一致 = %d", got.Total, want.Total)
	}
	for k, v := range want.Fine {
		if got.Fine[k] != v {
			t.Errorf("Fine[%d（%s）] = %d，期望 %d（乱序重复输入应与去重输入一致）", k, k, got.Fine[k], v)
		}
	}
	for i := range messy {
		if !messy[i].Equal(backup[i]) {
			t.Errorf("入参切片被修改：messy[%d] = %v，期望保持 %v", i, messy[i], backup[i])
		}
	}

	// 年份 0 哨兵回归：首日期年份为 0 时同样校验加载状态，返回错误而非 panic。
	if _, err := c.Stats([]time.Time{time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC)}, true); !errors.Is(err, goliday.ErrYearNotLoaded) {
		t.Errorf("Stats（首日期年份 0）错误 = %v，期望包装 ErrYearNotLoaded（不应 panic）", err)
	}
}

// slicesClone 测试内用的切片复制（保持入参不被修改的断言可对照）。
func slicesClone(ds []time.Time) []time.Time {
	out := make([]time.Time, len(ds))
	copy(out, ds)
	return out
}

// TestCoveredYears 导出的区间覆盖年份计算：空区间为空集、同年、跨年
// 含中间整年、终点恰为次年元旦时折叠进上一年（与库内校验同一实现）。
func TestCoveredYears(t *testing.T) {
	tests := []struct {
		name       string
		start, end string
		want       []int
	}{
		{"空区间 end==start", "2026-05-01", "2026-05-01", nil},
		{"倒置区间 end<start", "2026-05-02", "2026-05-01", nil},
		{"同年", "2026-02-01", "2026-02-28", []int{2026}},
		{"跨年", "2025-11-01", "2026-03-01", []int{2025, 2026}},
		{"终点为次年元旦折叠", "2025-11-01", "2026-01-01", []int{2025}},
		{"起点为元旦", "2026-01-01", "2027-01-01", []int{2026}},
	}
	for _, tt := range tests {
		got := goliday.CoveredYears(date(t, tt.start), date(t, tt.end))
		if len(got) != len(tt.want) {
			t.Fatalf("%s: CoveredYears = %v，期望 %v", tt.name, got, tt.want)
		}
		for i, y := range tt.want {
			if got[i] != y {
				t.Errorf("%s: CoveredYears[%d] = %d，期望 %d", tt.name, i, got[i], y)
			}
		}
	}
}

// TestStatsRangeMatchesBruteForce 前缀和 vs 暴力一致性：全年、随机子区间、
// 跨年区间，粗/细/Total 完全一致。
func TestStatsRangeMatchesBruteForce(t *testing.T) {
	c := newTestCalendar(t)

	ranges := []struct {
		name           string
		start, end     string
		expectedConfig bool // 是否显式给出期望总数（全年区间）
	}{
		{"2025 全年", "2025-01-01", "2026-01-01", true},
		{"2026 全年", "2026-01-01", "2027-01-01", true},
		{"2026 春节段", "2026-02-14", "2026-02-23", false},
		{"2026 年末跨年界", "2026-12-25", "2027-01-01", false},
		{"跨年 2025→2026", "2025-11-01", "2026-03-01", false},
		{"2025 元旦段", "2025-01-01", "2025-01-08", false},
		{"单日", "2026-10-01", "2026-10-02", false},
	}
	for _, rg := range ranges {
		s, e := date(t, rg.start), date(t, rg.end)
		for _, detailed := range []bool{false, true} {
			got, err := c.StatsRange(s, e, detailed)
			if err != nil {
				t.Fatalf("%s（detailed=%v）StatsRange 意外报错: %v", rg.name, detailed, err)
			}
			want := bruteForceStats(t, c, s, e, detailed)
			wantSameStats(t, fmt.Sprintf("%s detailed=%v", rg.name, detailed), got, want)
		}
	}
}

// TestStatsMatchesBruteForceList 离散列表（含跨年）前缀和 vs 暴力一致性。
func TestStatsMatchesBruteForceList(t *testing.T) {
	c := newTestCalendar(t)
	dates := []time.Time{
		date(t, "2025-12-31"), date(t, "2026-01-01"), date(t, "2026-01-02"),
		date(t, "2026-02-17"), date(t, "2026-02-28"),
	}
	for _, detailed := range []bool{false, true} {
		got, err := c.Stats(dates, detailed)
		if err != nil {
			t.Fatalf("Stats（detailed=%v）意外报错: %v", detailed, err)
		}
		want := goliday.StatsResult{Total: len(dates), Coarse: map[goliday.DayType]int{}}
		if detailed {
			want.Fine = map[goliday.DayType]int{}
		}
		for _, d := range dates {
			dt, err := c.Query(d)
			if err != nil {
				t.Fatalf("Query(%s) 意外报错: %v", d.Format(layout), err)
			}
			want.Coarse[dt.Coarse()]++
			if detailed {
				want.Fine[fineKeyOf(dt)]++
			}
		}
		wantSameStats(t, fmt.Sprintf("list detailed=%v", detailed), got, want)
	}
}

// TestStatsRangeEmpty 空区间与非法区间返回全零且不校验年份。
func TestStatsRangeEmpty(t *testing.T) {
	c := newTestCalendar(t)
	for _, rg := range [][2]string{
		{"2027-05-01", "2027-05-01"}, // 空区间：未加载年也不报错
		{"2026-05-01", "2025-05-01"}, // end < start
	} {
		r, err := c.StatsRange(date(t, rg[0]), date(t, rg[1]), true)
		if err != nil || r.Total != 0 || len(r.Coarse) != 0 || len(r.Fine) != 0 {
			t.Errorf("StatsRange(%v) = %+v, %v，期望全零且无错", rg, r, err)
		}
	}
}

// TestCalendarIsWorkIsRest 便捷方法与粗粒度判断一致性断言。
func TestCalendarIsWorkIsRest(t *testing.T) {
	c := newTestCalendar(t)

	tests := []struct {
		date string
		work bool
		rest bool
	}{
		{"2026-02-20", false, true},
		{"2026-02-21", false, true},
		{"2026-02-28", true, false},
		{"2026-03-03", true, false},
	}
	for _, tt := range tests {
		d := date(t, tt.date)
		if got, err := c.IsWork(d); err != nil || got != tt.work {
			t.Errorf("IsWork(%s) = %v, %v，期望 %v", tt.date, got, err, tt.work)
		}
		if got, err := c.IsRest(d); err != nil || got != tt.rest {
			t.Errorf("IsRest(%s) = %v, %v，期望 %v", tt.date, got, err, tt.rest)
		}
	}
}

// TestCalendarConfiguredYearExhaustive 对已配置年份（2025、2026）全年逐日
// 校验：结果恒属于 5 种合法细粒度值、粗细一致。
func TestCalendarConfiguredYearExhaustive(t *testing.T) {
	c := newTestCalendar(t)

	for _, year := range []int{2025, 2026} {
		start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		for i := range 366 {
			d := start.AddDate(0, 0, i)
			if d.Year() != year {
				break
			}
			got, err := c.Query(d)
			if err != nil {
				t.Fatalf("%s: Query 意外报错: %v", d.Format(layout), err)
			}
			if !legalFineValues[got] {
				t.Fatalf("%s: Query = %d（%s），不在合法值全集内", d.Format(layout), got, got)
			}
			gotC, err := c.QueryCoarse(d)
			if err != nil || gotC != got.Coarse() {
				t.Fatalf("%s: QueryCoarse 与 Query().Coarse() 不一致（%v）", d.Format(layout), err)
			}
			w, werr := c.IsWork(d)
			h, herr := c.IsRest(d)
			if werr != nil || herr != nil || w == h {
				t.Fatalf("%s: IsWork/IsRest 必须恰一为真（%v/%v）", d.Format(layout), werr, herr)
			}
		}
	}
}

// TestCalendarHasYear HasYear 与 Store.Years 一致。
func TestCalendarHasYear(t *testing.T) {
	c := newTestCalendar(t)
	for _, y := range []int{2025, 2026} {
		if !c.HasYear(y) {
			t.Errorf("HasYear(%d) = false，期望 true", y)
		}
	}
	for _, y := range []int{2024, 2027, 2030} {
		if c.HasYear(y) {
			t.Errorf("HasYear(%d) = true，期望 false", y)
		}
	}
}

// FuzzQueryConsistency 不变量：任意构造的时间点，Query 结果合法、粗细一致、
// 同一日不同时刻与时区表示结果不变；未加载年份返回 ErrYearNotLoaded。
func FuzzQueryConsistency(f *testing.F) {
	for _, seed := range [][5]int{
		{2026, 2, 17, 0, 0},
		{2026, 2, 28, 12, 30},
		{2025, 1, 29, 23, 59},
		{2027, 5, 1, 6, 0},
		{2026, 4, 5, 18, 45},
	} {
		f.Add(seed[0], seed[1], seed[2], seed[3], seed[4])
	}
	f.Fuzz(func(t *testing.T, year, month, day, hour, minute int) {
		if year < 1 || year > 9999 || month < 1 || month > 12 {
			t.Skip()
		}
		// 归一化 day/hour/minute 到 time.Date 可表示域，避免构造出下月日期。
		if day < 1 {
			day = 1
		}
		if day > 28 {
			day = 28
		}
		hour %= 24
		minute %= 60
		if hour < 0 {
			hour += 24
		}
		if minute < 0 {
			minute += 60
		}
		ts := time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.UTC)

		c := newTestCalendar(t)
		if !c.HasYear(year) {
			for _, fn := range []func() error{
				func() error { _, err := c.Query(ts); return err },
				func() error { _, err := c.QueryCoarse(ts); return err },
				func() error { _, err := c.IsWork(ts); return err },
				func() error { _, err := c.IsRest(ts); return err },
			} {
				if err := fn(); !errors.Is(err, goliday.ErrYearNotLoaded) {
					t.Fatalf("未加载年 %s 应返回 ErrYearNotLoaded，got %v", ts.Format(layout), err)
				}
			}
			return
		}

		got, err := c.Query(ts)
		if err != nil {
			t.Fatalf("Query(%s) 意外报错: %v", ts.Format(layout), err)
		}
		if !legalFineValues[got] {
			t.Fatalf("Query(%s) = %d（%s），不在合法值全集内", ts.Format(layout), got, got)
		}
		if coarse, err := c.QueryCoarse(ts); err != nil || coarse != got.Coarse() {
			t.Fatalf("QueryCoarse(%s) 与 Query().Coarse() 不一致（%v）", ts.Format(layout), err)
		}
		if w, werr := c.IsWork(ts); werr != nil || w == got.IsRest() {
			t.Fatalf("IsWork/IsRest(%s) 必须恰一为真", ts.Format(layout))
		}

		// 同一日不同时刻/时区表示不变（基于当日午夜构造，确保不跨日）。
		midnight := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		base, err := c.Query(midnight)
		if err != nil {
			t.Fatalf("Query(%s) 意外报错: %v", midnight.Format(layout), err)
		}
		for _, v := range []time.Time{
			midnight.Add(5 * time.Hour),
			midnight.Add(23 * time.Hour),
			time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.FixedZone("CST", 8*3600)),
		} {
			if got2, err := c.Query(v); err != nil || got2 != base {
				t.Fatalf("同一日 %s 的另一表示 %s 判型不一致：%d vs %d（%v）",
					midnight.Format(layout), v.Format("2006-01-02 15:04 MST"), base, got2, err)
			}
		}
	})
}
