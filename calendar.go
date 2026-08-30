package goliday

import "time"

// normalizeDate 将任意时刻规范化为其所在日的 UTC 午夜零点，
// 作为索引构建与查询共用的日期键，以消除时刻与时区差异。
func normalizeDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// yearIndex 单一年份配置的快速查找索引，键为规范化到 UTC 午夜的日期。
type yearIndex struct {
	off      map[time.Time]struct{}
	work     map[time.Time]struct{}
	festival map[time.Time]struct{}
}

// Calendar 提供日期类型查询能力。由 Store 一次性构建各年份查找索引，
// 构造完成后只读，可被多个 goroutine 并发调用。
type Calendar struct {
	years map[int]*yearIndex
}

// NewCalendar 将 Store 中各年份的稀疏配置转换为按日期哈希的快速查找索引。
func NewCalendar(s *Store) *Calendar {
	c := &Calendar{years: make(map[int]*yearIndex)}
	for _, y := range s.Years() {
		cfg := s.Get(y)
		if cfg == nil {
			continue
		}
		idx := &yearIndex{
			off:      make(map[time.Time]struct{}, len(cfg.Adjust.Off)),
			work:     make(map[time.Time]struct{}, len(cfg.Adjust.Work)),
			festival: make(map[time.Time]struct{}, len(cfg.Festivals)),
		}
		for _, d := range cfg.Adjust.Off {
			idx.off[normalizeDate(d)] = struct{}{}
		}
		for _, d := range cfg.Adjust.Work {
			idx.work[normalizeDate(d)] = struct{}{}
		}
		for _, f := range cfg.Festivals {
			idx.festival[normalizeDate(f.Date)] = struct{}{}
		}
		c.years[y] = idx
	}
	return c
}

// Query 返回 date 的细粒度日期类型。任意时刻均先按其所在日规范化再查询。
//
// 判断顺序：周休回退（周六/周日 → Weekend，否则 Ordinary）；该年有配置时
// off 命中 → Adjusted、work 命中 → Compensate|Weekend；节日当天追加
// Festival 位。该年无配置则整年退回纯周休判断。
func (c *Calendar) Query(date time.Time) DayType {
	d := normalizeDate(date)

	t := DayTypeOrdinary
	if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
		t = DayTypeWeekend
	}

	idx := c.years[d.Year()]
	if idx == nil {
		return t // 无该年配置：仅周休回退
	}
	if _, ok := idx.off[d]; ok {
		t = DayTypeAdjusted
	}
	if _, ok := idx.work[d]; ok {
		t = DayTypeCompensate | DayTypeWeekend
	}
	if _, ok := idx.festival[d]; ok {
		t |= DayTypeFestival
	}
	return t
}

// QueryCoarse 返回 date 的粗粒度日期类型（等价于 Query(date).Coarse()）。
func (c *Calendar) QueryCoarse(date time.Time) DayType {
	return c.Query(date).Coarse()
}

// IsWorkday 报告 date 是否为上班日。
func (c *Calendar) IsWorkday(date time.Time) bool { return c.Query(date).IsWorkday() }

// IsHoliday 报告 date 是否为休息日。
func (c *Calendar) IsHoliday(date time.Time) bool { return c.Query(date).IsHoliday() }

// Dated 区间/列表查询结果中的单日条目：Date 为规范化到 UTC 午夜的日期，
// Type 为细粒度日期类型。
type Dated struct {
	Date time.Time
	Type DayType
}

// QueryRange 逐日返回左闭右开区间 [start, end) 内每天的细粒度类型。
//
// end 早于或等于 start 时返回空切片。本方法不限制区间跨度，
// 跨度上限校验由调用方（如 HTTP 层）负责。
func (c *Calendar) QueryRange(start, end time.Time) []Dated {
	s, e := normalizeDate(start), normalizeDate(end)
	days := 0
	if diff := e.Sub(s); diff > 0 {
		days = int(diff.Hours()/24) + 1
	}
	out := make([]Dated, 0, days)
	for d := s; d.Before(e); d = d.AddDate(0, 0, 1) {
		out = append(out, Dated{Date: d, Type: c.Query(d)})
	}
	return out
}

// StatsResult 统计结果：
//
//	Total  输入日期总数（= len(dates)，不做去重）；
//	Coarse 粗粒度计数，键为 DayTypeWorkday / DayTypeHoliday；
//	Fine   细粒度单标志位交叉计数，组合日（如 Festival|Adjusted）对其含有的
//	       每个标志位各计 1，各键之和可大于 Total；detailed=false 时为 nil。
type StatsResult struct {
	Total  int
	Coarse map[DayType]int
	Fine   map[DayType]int
}

// Stats 对 dates 逐日 Query 并统计。dates 视为已去重的日期集合
// （去重与排序由调用方负责），逐日统计不去重。
//
// detailed=false 仅返回粗粒度 workday/holiday 计数；detailed=true 额外返回
// 细粒度交叉计数。
func (c *Calendar) Stats(dates []time.Time, detailed bool) StatsResult {
	r := StatsResult{
		Total:  len(dates),
		Coarse: make(map[DayType]int, 2),
	}
	if detailed {
		r.Fine = make(map[DayType]int, len(dayTypeNames))
	}
	for _, d := range dates {
		t := c.Query(d)
		r.Coarse[t.Coarse()]++
		if detailed {
			for i := range dayTypeNames {
				bit := DayType(1) << i
				if t&bit != 0 {
					r.Fine[bit]++
				}
			}
		}
	}
	return r
}
