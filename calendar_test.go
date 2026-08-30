package goliday

import (
	"testing"
	"time"
)

// newTestCalendar 加载 testdata 真实配置（2025、2026）构造 Calendar。
func newTestCalendar(t *testing.T) *Calendar {
	t.Helper()
	return NewCalendar(mustLoadDir(t, "testdata"))
}

// TestCalendarQuery 单日细/粗粒度查询断言（2026 假设方案、2025 真实数据、无配置年份回退）。
func TestCalendarQuery(t *testing.T) {
	c := newTestCalendar(t)

	tests := []struct {
		name   string
		date   string
		want   DayType
		coarse DayType
	}{
		{"2026-02-20 周五调休off", "2026-02-20", DayTypeAdjusted, DayTypeHoliday},
		{"2026-02-17 春节当天且off", "2026-02-17", DayTypeFestival | DayTypeAdjusted, DayTypeHoliday},
		{"2026-02-28 周六补班work", "2026-02-28", DayTypeCompensate | DayTypeWeekend, DayTypeWorkday},
		{"2026-04-05 周日清明当天不在off/work", "2026-04-05", DayTypeFestival | DayTypeWeekend, DayTypeHoliday},
		{"2026-03-03 周二未覆盖", "2026-03-03", DayTypeOrdinary, DayTypeWorkday},
		{"2026-02-21 周六春节假期内自然周末", "2026-02-21", DayTypeWeekend, DayTypeHoliday},
		{"2027-05-01 周六无2027配置", "2027-05-01", DayTypeWeekend, DayTypeHoliday},
		{"2027-03-02 周一无配置", "2027-03-02", DayTypeOrdinary, DayTypeWorkday},
		{"2025-01-26 周日补班", "2025-01-26", DayTypeCompensate | DayTypeWeekend, DayTypeWorkday},
		{"2025-01-29 正月初一", "2025-01-29", DayTypeFestival | DayTypeAdjusted, DayTypeHoliday},
		{"2025-10-06 中秋当天周一", "2025-10-06", DayTypeFestival | DayTypeAdjusted, DayTypeHoliday},
		{"2025-04-05 周六假期内自然周末", "2025-04-05", DayTypeWeekend, DayTypeHoliday},
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

	want := DayTypeFestival | DayTypeAdjusted
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
	want := []Dated{
		{date(t, "2026-02-14"), DayTypeCompensate | DayTypeWeekend},
		{date(t, "2026-02-15"), DayTypeWeekend},
		{date(t, "2026-02-16"), DayTypeAdjusted},
	}
	if len(got) != len(want) {
		t.Fatalf("QueryRange 返回 %d 个元素，期望 %d（左闭右开不含 end）：%v", len(got), len(want), got)
	}
	for i, w := range want {
		if !got[i].Date.Equal(w.Date) || got[i].Type != w.Type {
			t.Errorf("days[%d] = {%s %d（%s）}，期望 {%s %d（%s）}",
				i, got[i].Date.Format(dateLayout), got[i].Type, got[i].Type,
				w.Date.Format(dateLayout), w.Type, w.Type)
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
	if r.Coarse[DayTypeWorkday] != 2 || r.Coarse[DayTypeHoliday] != 1 {
		t.Errorf("Coarse = %v，期望 workday=2、holiday=1", r.Coarse)
	}
	wantFine := map[DayType]int{
		DayTypeOrdinary:   1,
		DayTypeCompensate: 1,
		DayTypeWeekend:    1,
		DayTypeFestival:   1,
		DayTypeAdjusted:   1,
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
	if rc.Coarse[DayTypeWorkday] != 2 || rc.Coarse[DayTypeHoliday] != 1 {
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
