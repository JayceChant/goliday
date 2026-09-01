// 白盒测试（package goliday）：覆盖统计内部辅助的分支与兜底路径——
// 非法细粒度组合的 comboIndex、Store 返回 nil 配置时 NewCalendar 的跳过。
package goliday

import (
	"testing"
	"time"
)

// TestComboIndexInvalid 非法组合（合法位集合之外的任意值）返回 -1，
// 前缀和构建时该日不落入任何计数桶。
func TestComboIndexInvalid(t *testing.T) {
	for _, v := range comboValues {
		if got := comboIndex(v); got < 0 {
			t.Errorf("comboIndex(%d) = %d，期望命中", v, got)
		}
	}
	for _, v := range []DayType{
		0,
		DayTypeOrdinary | DayTypeAdjusted,   // 17：工作日与调休冲突
		DayTypeCompensate,                   // 2：补班缺周末位
		DayTypeCompensate | DayTypeFestival, // 10：非法组合
		DayTypeWeekend | DayTypeAdjusted,    // 20：非法组合
		DayTypeCompensate | DayTypeWeekend | DayTypeAdjusted, // 22：非法组合
	} {
		if got := comboIndex(v); got != -1 {
			t.Errorf("comboIndex(%d) = %d，期望 -1", v, got)
		}
	}
}

// TestNewCalendarSkipsNilConfig Store 中 Get 返回 nil（年份被并发移除等
// 外部异常，实际加载路径不会发生）时，NewCalendar 跳过该年份不 panic。
func TestNewCalendarSkipsNilConfig(t *testing.T) {
	s := &Store{years: map[int]*YearConfig{2026: nil}}
	c := NewCalendar(s)
	if c.HasYear(2026) {
		t.Error("nil 配置年份不应被建索引")
	}
	if _, err := c.Query(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Error("nil 配置年份查询应报错")
	}
}
