// 黑盒测试（package goliday_test）：仅经导出 API 验证 Calendar 判定、
// 区间与统计的对外契约（判断算法、粗细映射、无配置年回退）。
package goliday_test

import (
	"testing"
	"time"

	"goliday"
)

// newTestCalendar 加载 testdata 真实配置（2025、2026）构造 Calendar。
func newTestCalendar(t *testing.T) *goliday.Calendar {
	t.Helper()
	return goliday.NewCalendar(mustLoadDir(t, "testdata"))
}

// TestCalendarQuery 单日细/粗粒度查询断言（2026 假设方案、2025 真实数据、无配置年份回退）。
func TestCalendarQuery(t *testing.T) {
	c := newTestCalendar(t)

	tests := []struct {
		name   string
		date   string
		want   goliday.DayType
		coarse goliday.DayType
	}{
		{"2026-02-20 周五调休off", "2026-02-20", goliday.DayTypeAdjusted, goliday.DayTypeHoliday},
		{"2026-02-17 春节当天且off", "2026-02-17", goliday.DayTypeFestival | goliday.DayTypeAdjusted, goliday.DayTypeHoliday},
		{"2026-02-28 周六补班work", "2026-02-28", goliday.DayTypeCompensate | goliday.DayTypeWeekend, goliday.DayTypeWorkday},
		{"2026-04-05 周日清明当天不在off/work", "2026-04-05", goliday.DayTypeFestival | goliday.DayTypeWeekend, goliday.DayTypeHoliday},
		{"2026-03-03 周二未覆盖", "2026-03-03", goliday.DayTypeOrdinary, goliday.DayTypeWorkday},
		{"2026-02-21 周六春节假期内自然周末", "2026-02-21", goliday.DayTypeWeekend, goliday.DayTypeHoliday},
		{"2027-05-01 周六无2027配置", "2027-05-01", goliday.DayTypeWeekend, goliday.DayTypeHoliday},
		{"2027-03-02 周一无配置", "2027-03-02", goliday.DayTypeOrdinary, goliday.DayTypeWorkday},
		{"2025-01-26 周日补班", "2025-01-26", goliday.DayTypeCompensate | goliday.DayTypeWeekend, goliday.DayTypeWorkday},
		{"2025-01-29 正月初一", "2025-01-29", goliday.DayTypeFestival | goliday.DayTypeAdjusted, goliday.DayTypeHoliday},
		{"2025-10-06 中秋当天周一", "2025-10-06", goliday.DayTypeFestival | goliday.DayTypeAdjusted, goliday.DayTypeHoliday},
		{"2025-04-05 周六假期内自然周末", "2025-04-05", goliday.DayTypeWeekend, goliday.DayTypeHoliday},
	}
	for _, tt := range tests {
		d := date(t, tt.date)
		if got := c.Query(d); got != tt.want {
			t.Errorf("%s: Query = %d（%s），期望 %d（%s）", tt.name, got, got, tt.want, tt.want)
		}
		if got := c.QueryCoarse(d); got != tt.coarse {
			t.Errorf("%s: QueryCoarse = %d（%s），期望 %d（%s）", tt.name, got, got, tt.coarse, tt.coarse)
		}
	}
}

// TestCalendarQueryNormalization 同一日不同时刻/时区的输入应得到一致结果。
func TestCalendarQueryNormalization(t *testing.T) {
	c := newTestCalendar(t)

	want := goliday.DayTypeFestival | goliday.DayTypeAdjusted
	variants := []time.Time{
		date(t, "2026-02-17"),
		date(t, "2026-02-17").Add(15*time.Hour + 30*time.Minute),
		time.Date(2026, 2, 17, 23, 59, 59, 0, time.FixedZone("CST", 8*3600)),
	}
	for i, v := range variants {
		if got := c.Query(v); got != want {
			t.Errorf("variants[%d]（%s）Query = %d（%s），期望 %d（%s）",
				i, v.Format("2006-01-02 15:04:05 MST"), got, got, want, want)
		}
	}
}

// TestCalendarQueryRange 区间查询：左闭右开逐日、end<=start 返回空切片。
func TestCalendarQueryRange(t *testing.T) {
	c := newTestCalendar(t)

	got := c.QueryRange(date(t, "2026-02-14"), date(t, "2026-02-17"))
	want := []goliday.Dated{
		{date(t, "2026-02-14"), goliday.DayTypeCompensate | goliday.DayTypeWeekend},
		{date(t, "2026-02-15"), goliday.DayTypeWeekend},
		{date(t, "2026-02-16"), goliday.DayTypeAdjusted},
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

	if d := c.QueryRange(date(t, "2026-02-14"), date(t, "2026-02-14")); len(d) != 0 {
		t.Errorf("start == end 应返回空切片，got %v", d)
	}
	if d := c.QueryRange(date(t, "2026-02-17"), date(t, "2026-02-14")); len(d) != 0 {
		t.Errorf("end < start 应返回空切片，got %v", d)
	}
}

// TestCalendarStats 统计：细粒度交叉计数与粗粒度模式断言。
func TestCalendarStats(t *testing.T) {
	c := newTestCalendar(t)
	dates := []time.Time{date(t, "2026-02-17"), date(t, "2026-02-28"), date(t, "2026-03-03")}

	r := c.Stats(dates, true)
	if r.Total != 3 {
		t.Errorf("Total = %d，期望 3", r.Total)
	}
	if r.Coarse[goliday.DayTypeWorkday] != 2 || r.Coarse[goliday.DayTypeHoliday] != 1 {
		t.Errorf("Coarse = %v，期望 workday=2、holiday=1", r.Coarse)
	}
	wantFine := map[goliday.DayType]int{
		goliday.DayTypeOrdinary:   1,
		goliday.DayTypeCompensate: 1,
		goliday.DayTypeWeekend:    1,
		goliday.DayTypeFestival:   1,
		goliday.DayTypeAdjusted:   1,
	}
	if len(r.Fine) != len(wantFine) {
		t.Fatalf("Fine = %v，期望 %v", r.Fine, wantFine)
	}
	for k, v := range wantFine {
		if r.Fine[k] != v {
			t.Errorf("Fine[%s] = %d，期望 %d", k, r.Fine[k], v)
		}
	}

	rc := c.Stats(dates, false)
	if rc.Total != 3 {
		t.Errorf("粗粒度模式 Total = %d，期望 3", rc.Total)
	}
	if rc.Coarse[goliday.DayTypeWorkday] != 2 || rc.Coarse[goliday.DayTypeHoliday] != 1 {
		t.Errorf("粗粒度模式 Coarse = %v，期望 workday=2、holiday=1", rc.Coarse)
	}
	if len(rc.Fine) != 0 {
		t.Errorf("粗粒度模式 Fine 应无计数，got %v", rc.Fine)
	}

	if r := c.Stats(nil, true); r.Total != 0 || len(r.Coarse) != 0 || len(r.Fine) != 0 {
		t.Errorf("空输入 Stats = %+v，期望全零", r)
	}
}

// TestCalendarIsWorkdayIsHoliday 便捷方法与粗粒度判断一致性断言。
func TestCalendarIsWorkdayIsHoliday(t *testing.T) {
	c := newTestCalendar(t)

	tests := []struct {
		date    string
		workday bool
		holiday bool
	}{
		{"2026-02-20", false, true},
		{"2026-02-21", false, true},
		{"2026-02-28", true, false},
		{"2026-03-03", true, false},
		{"2027-03-02", true, false},
	}
	for _, tt := range tests {
		d := date(t, tt.date)
		if got := c.IsWorkday(d); got != tt.workday {
			t.Errorf("IsWorkday(%s) = %v，期望 %v", tt.date, got, tt.workday)
		}
		if got := c.IsHoliday(d); got != tt.holiday {
			t.Errorf("IsHoliday(%s) = %v，期望 %v", tt.date, got, tt.holiday)
		}
	}
}

// TestCalendarConfiguredYearExhaustive 对已配置年份（2025、2026）全年逐日
// 校验：结果恒属于 6 种合法细粒度组合、粗细一致、无配置的 2027 全年
// 仅 Ordinary/Weekend。
func TestCalendarConfiguredYearExhaustive(t *testing.T) {
	c := newTestCalendar(t)

	for _, year := range []int{2025, 2026, 2027} {
		start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		for i := 0; i < 366; i++ {
			d := start.AddDate(0, 0, i)
			if d.Year() != year {
				break
			}
			got := c.Query(d)
			if !legalCombos[got] {
				t.Fatalf("%s: Query = %d（%s），不在合法组合全集内", d.Format(layout), got, got)
			}
			if c.QueryCoarse(d) != got.Coarse() {
				t.Fatalf("%s: QueryCoarse 与 Query().Coarse() 不一致", d.Format(layout))
			}
			if c.IsWorkday(d) == c.IsHoliday(d) {
				t.Fatalf("%s: IsWorkday/IsHoliday 必须恰一为真", d.Format(layout))
			}
			if year == 2027 && got != goliday.DayTypeOrdinary && got != goliday.DayTypeWeekend {
				t.Fatalf("无配置年 2027 的 %s 判型 = %d（%s），应仅 Ordinary/Weekend", d.Format(layout), got, got)
			}
		}
	}
}

// FuzzQueryConsistency 不变量：任意构造的时间点，Query 结果合法、粗细一致、
// 同一日不同时刻与时区表示结果不变；无配置年份仅 Ordinary/Weekend。
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
		got := c.Query(ts)
		if !legalCombos[got] {
			t.Fatalf("Query(%s) = %d（%s），不在合法组合全集内", ts.Format(layout), got, got)
		}
		if coarse := c.QueryCoarse(ts); coarse != got.Coarse() {
			t.Fatalf("QueryCoarse(%s) = %d，Query().Coarse() = %d", ts.Format(layout), coarse, got.Coarse())
		}
		if c.IsWorkday(ts) == c.IsHoliday(ts) {
			t.Fatalf("IsWorkday/IsHoliday(%s) 必须恰一为真", ts.Format(layout))
		}

		// 同一日不同时刻/时区表示不变（基于当日午夜构造，确保不跨日）。
		midnight := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		base := c.Query(midnight)
		for _, v := range []time.Time{
			midnight.Add(5 * time.Hour),
			midnight.Add(23 * time.Hour),
			time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.FixedZone("CST", 8*3600)),
		} {
			if got2 := c.Query(v); got2 != base {
				t.Fatalf("同一日 %s 的另一表示 %s 判型不一致：%d vs %d",
					midnight.Format(layout), v.Format("2006-01-02 15:04 MST"), base, got2)
			}
		}

		// 无配置年份（2027 之外再取 2030）仅周休回退。
		for _, y := range []int{2027, 2030} {
			d := time.Date(y, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			wd := d.Weekday()
			want := goliday.DayTypeOrdinary
			if wd == time.Saturday || wd == time.Sunday {
				want = goliday.DayTypeWeekend
			}
			if got := c.Query(d); got != want {
				t.Fatalf("无配置年 %s 判型 = %d（%s），期望 %d（%s）", d.Format(layout), got, got, want, want)
			}
		}
	})
}
