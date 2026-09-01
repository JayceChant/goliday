// Package goliday 提供中国法定节假日与调休工作日的日期类型判定能力。
package goliday

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ErrYearNotLoaded 查询覆盖了未加载配置的年份。
// 未加载年份不再回退到系统周休判断，而是返回包装本错误的错误
// （errors.Is 可判别），错误消息中包含具体年份。
var ErrYearNotLoaded = errors.New("年份配置未加载")

// normalizeDate 将任意时刻规范化为其所在日的 UTC 午夜零点，
// 作为索引构建与查询共用的日期键，以消除时刻与时区差异。
func normalizeDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// 细粒度合法组合在 comboCounts 中的固定下标（组合值见 comboValues）。
const (
	comboOrdinary = iota // 1  Ordinary
	comboWeekend         // 4  Weekend
	comboCompWeek        // 6  Compensate|Weekend
	comboFestWeek        // 12 Festival|Weekend
	comboAdjusted        // 16 Adjusted
	comboFestAdj         // 24 Festival|Adjusted
)

// comboValues 与 comboCounts 下标一一对应的合法细粒度组合值，升序。
var comboValues = [...]DayType{
	DayTypeOrdinary,
	DayTypeWeekend,
	DayTypeCompensate | DayTypeWeekend,
	DayTypeFestival | DayTypeWeekend,
	DayTypeAdjusted,
	DayTypeFestival | DayTypeAdjusted,
}

// comboIndex 返回细粒度组合值在 comboCounts 中的下标；非合法组合返回 -1。
func comboIndex(t DayType) int {
	for i, v := range comboValues {
		if v == t {
			return i
		}
	}
	return -1
}

// comboCounts 6 种合法细粒度组合各自的累计天数（按组合计数，
// 非标志位交叉计数）。粗粒度与标志位交叉计数均可由其线性组合导出。
type comboCounts [len(comboValues)]int

// add 累加另一组计数。
func (c *comboCounts) add(o comboCounts) {
	for i := range o {
		c[i] += o[i]
	}
}

// diff 返回两组前缀计数的差（b-a，要求 b 的前缀位置不早于 a）。
func diffPrefix(b, a comboCounts) comboCounts {
	var out comboCounts
	for i := range b {
		out[i] = b[i] - a[i]
	}
	return out
}

// result 将组合累计计数导出为统计结果：细粒度为标志位交叉计数
// （组合日对其含有的每个标志各计 1），粗粒度按细→粗映射归并
// （补班优先归上班日）。detailed=false 时 Fine 为 nil。
func (c comboCounts) result(total int, detailed bool) StatsResult {
	r := StatsResult{
		Total: total,
		Coarse: map[DayType]int{
			DayTypeWorkday: c[comboOrdinary] + c[comboCompWeek],
			DayTypeHoliday: c[comboWeekend] + c[comboFestWeek] + c[comboAdjusted] + c[comboFestAdj],
		},
	}
	if detailed {
		r.Fine = map[DayType]int{
			DayTypeOrdinary:   c[comboOrdinary],
			DayTypeCompensate: c[comboCompWeek],
			DayTypeWeekend:    c[comboWeekend] + c[comboCompWeek] + c[comboFestWeek],
			DayTypeFestival:   c[comboFestWeek] + c[comboFestAdj],
			DayTypeAdjusted:   c[comboAdjusted] + c[comboFestAdj],
		}
	}
	return r
}

// yearIndex 单一年份配置的快速查找索引与统计前缀和，键为规范化到
// UTC 午夜的日期。
type yearIndex struct {
	off      map[time.Time]struct{}
	work     map[time.Time]struct{}
	festival map[time.Time]struct{}

	// first 该年元旦（UTC 午夜）；days 该年天数（365/366）。
	first time.Time
	days  int
	// prefix 细粒度组合计数前缀和，长度 days+1：prefix[0] 为全零，
	// prefix[i] 为 [元旦, 元旦+i天)（左闭右开）的组合累计，
	// 因此年内区间 [a, b) 的统计即 prefix[idx(b)] - prefix[idx(a)]。
	// 构建完成后只读，可被多个 goroutine 并发访问。
	prefix []comboCounts
}

// dayType 返回该年某日（已规范化）的细粒度类型。
//
// 判断顺序：周休回退（周六/周日 → Weekend，否则 Ordinary）；
// off 命中 → Adjusted、work 命中 → Compensate|Weekend；节日当天追加
// Festival 位。
func (idx *yearIndex) dayType(d time.Time) DayType {
	t := DayTypeOrdinary
	if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
		t = DayTypeWeekend
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

// dayIndex 返回该年某日（已规范化）距元旦的天数下标（0-based）。
func (idx *yearIndex) dayIndex(d time.Time) int {
	return int(d.Sub(idx.first).Hours() / 24)
}

// nextYearStart 返回该年次年的元旦（UTC 午夜）。
func (idx *yearIndex) nextYearStart() time.Time {
	return idx.first.AddDate(1, 0, 0)
}

// comboRange 返回年内左闭右开区间 [a, b) 的组合计数（b 可为次年元旦）。
func (idx *yearIndex) comboRange(a, b time.Time) comboCounts {
	return diffPrefix(idx.prefix[idx.dayIndex(b)], idx.prefix[idx.dayIndex(a)])
}

// buildPrefix 逐日判定并构建该年的组合计数前缀和。
func (idx *yearIndex) buildPrefix() {
	idx.prefix = make([]comboCounts, idx.days+1)
	for i := 1; i <= idx.days; i++ {
		idx.prefix[i] = idx.prefix[i-1]
		idx.prefix[i][comboIndex(idx.dayType(idx.first.AddDate(0, 0, i-1)))]++
	}
}

// yearDays 返回指定年份的天数（365/366）。
func yearDays(year int) int {
	y := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	next := y.AddDate(1, 0, 0)
	return int(next.Sub(y).Hours() / 24)
}

// Calendar 提供日期类型查询与统计能力。由 Store 一次性构建各年份的
// 查找索引与组合计数前缀和，构造完成后只读，可被多个 goroutine 并发调用。
type Calendar struct {
	years map[int]*yearIndex
}

// NewCalendar 将 Store 中各年份的稀疏配置转换为按日期哈希的快速查找
// 索引，并为每年构建细粒度组合计数前缀和（供 StatsRange/Stats 差分统计）。
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
			first:    time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC),
			days:     yearDays(y),
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
		idx.buildPrefix()
		c.years[y] = idx
	}
	return c
}

// HasYear 报告指定年份的配置是否已加载。
func (c *Calendar) HasYear(year int) bool {
	_, ok := c.years[year]
	return ok
}

// yearNotLoadedErr 构造包含全部未加载年份（升序）的包装错误。
func yearNotLoadedErr(years []int) error {
	names := make([]string, len(years))
	for i, y := range years {
		names[i] = strconv.Itoa(y)
	}
	return fmt.Errorf("%w: %s", ErrYearNotLoaded, strings.Join(names, ", "))
}

// indexFor 返回日期所在年的索引；该年未加载时返回包装 ErrYearNotLoaded
// 的错误。
func (c *Calendar) indexFor(d time.Time) (*yearIndex, error) {
	idx := c.years[d.Year()]
	if idx == nil {
		return nil, fmt.Errorf("%w: %d", ErrYearNotLoaded, d.Year())
	}
	return idx, nil
}

// missingYears 返回 years 中未加载的年份（升序去重后仍保持升序；
// 要求输入升序去重）。全部已加载时返回 nil。
func (c *Calendar) missingYears(years []int) []int {
	var missing []int
	for _, y := range years {
		if !c.HasYear(y) {
			missing = append(missing, y)
		}
	}
	return missing
}

// coveredYears 返回左闭右开区间 [s, e)（已规范化，e 晚于 s）覆盖的全部
// 年份（升序，含仅被部分覆盖的中间整年）。
func coveredYears(s, e time.Time) []int {
	first, last := s.Year(), e.AddDate(0, 0, -1).Year()
	years := make([]int, 0, last-first+1)
	for y := first; y <= last; y++ {
		years = append(years, y)
	}
	return years
}

// Query 返回 date 的细粒度日期类型。任意时刻均先按其所在日规范化再查询。
//
// 判断顺序：周休回退（周六/周日 → Weekend，否则 Ordinary）；该年有配置时
// off 命中 → Adjusted、work 命中 → Compensate|Weekend；节日当天追加
// Festival 位。该年未加载配置时返回包装 ErrYearNotLoaded 的错误，
// 不再回退周休判断。
func (c *Calendar) Query(date time.Time) (DayType, error) {
	d := normalizeDate(date)
	idx, err := c.indexFor(d)
	if err != nil {
		return 0, err
	}
	return idx.dayType(d), nil
}

// QueryCoarse 返回 date 的粗粒度日期类型（等价于 Query(date).Coarse()）。
func (c *Calendar) QueryCoarse(date time.Time) (DayType, error) {
	t, err := c.Query(date)
	if err != nil {
		return 0, err
	}
	return t.Coarse(), nil
}

// IsWorkday 报告 date 是否为上班日。
func (c *Calendar) IsWorkday(date time.Time) (bool, error) {
	t, err := c.Query(date)
	if err != nil {
		return false, err
	}
	return t.IsWorkday(), nil
}

// IsHoliday 报告 date 是否为休息日。
func (c *Calendar) IsHoliday(date time.Time) (bool, error) {
	t, err := c.Query(date)
	if err != nil {
		return false, err
	}
	return t.IsHoliday(), nil
}

// Dated 区间/列表查询结果中的单日条目：Date 为规范化到 UTC 午夜的日期，
// Type 为细粒度日期类型。
type Dated struct {
	Date time.Time
	Type DayType
}

// QueryRange 逐日返回左闭右开区间 [start, end) 内每天的细粒度类型。
//
// end 早于或等于 start 时返回空切片（空区间无覆盖年份，不校验加载状态）。
// 覆盖任一年份未加载时返回包装 ErrYearNotLoaded 的错误。本方法不限制
// 区间跨度，跨度上限校验由调用方（如 HTTP 层）负责。
func (c *Calendar) QueryRange(start, end time.Time) ([]Dated, error) {
	s, e := normalizeDate(start), normalizeDate(end)
	if !e.After(s) {
		return []Dated{}, nil
	}
	if missing := c.missingYears(coveredYears(s, e)); missing != nil {
		return nil, yearNotLoadedErr(missing)
	}
	out := make([]Dated, 0, int(e.Sub(s).Hours()/24)+1)
	for d := s; d.Before(e); d = d.AddDate(0, 0, 1) {
		out = append(out, Dated{Date: d, Type: c.years[d.Year()].dayType(d)})
	}
	return out, nil
}

// StatsResult 统计结果：
//
//	Total  覆盖天数（区间天数或 len(dates)，不做去重）；
//	Coarse 粗粒度计数，键为 DayTypeWorkday / DayTypeHoliday；
//	Fine   细粒度单标志位交叉计数，组合日（如 Festival|Adjusted）对其含有的
//	       每个标志位各计 1，各键之和可大于 Total；detailed=false 时为 nil。
type StatsResult struct {
	Total  int
	Coarse map[DayType]int
	Fine   map[DayType]int
}

// StatsRange 统计左闭右开区间 [start, end)（不限跨度）。
//
// 基于构造期构建的组合计数前缀和差分：年内 O(1)，跨年按
// 「首年段 + 若干整年段 + 末年段」拆分后逐段差分相加，复杂度 O(覆盖年数)。
// 覆盖任一年份未加载时返回包装 ErrYearNotLoaded 的错误；
// end 早于或等于 start 时返回全零结果（空区间无覆盖年份，不校验）。
func (c *Calendar) StatsRange(start, end time.Time, detailed bool) (StatsResult, error) {
	s, e := normalizeDate(start), normalizeDate(end)
	if !e.After(s) {
		return StatsResult{Coarse: map[DayType]int{}}, nil
	}
	if missing := c.missingYears(coveredYears(s, e)); missing != nil {
		return StatsResult{}, yearNotLoadedErr(missing)
	}

	var acc comboCounts
	total := 0
	for d := s; d.Before(e); {
		idx := c.years[d.Year()]
		segEnd := idx.nextYearStart()
		if e.Before(segEnd) {
			segEnd = e
		}
		acc.add(idx.comboRange(d, segEnd))
		total += int(segEnd.Sub(d).Hours() / 24)
		d = segEnd
	}
	return acc.result(total, detailed), nil
}

// Stats 统计日期集合。dates 视为已去重升序的日期集合（去重与排序由
// 调用方负责），逐元素计数不去重。
//
// 实现上逐日取前缀和的单日差分（prefix[i+1] - prefix[i]），复用与
// StatsRange 相同的前缀和数据。任一日期所在年份未加载时返回包装
// ErrYearNotLoaded 的错误；空输入直接返回全零结果。
func (c *Calendar) Stats(dates []time.Time, detailed bool) (StatsResult, error) {
	if len(dates) == 0 {
		return StatsResult{Coarse: map[DayType]int{}}, nil
	}

	// 年份校验：dates 已升序，年份按序首见时收集。
	var missing []int
	prev := 0
	for _, d := range dates {
		y := normalizeDate(d).Year()
		if y != prev {
			if !c.HasYear(y) {
				missing = append(missing, y)
			}
			prev = y
		}
	}
	if missing != nil {
		return StatsResult{}, yearNotLoadedErr(missing)
	}

	var acc comboCounts
	for _, d := range dates {
		dn := normalizeDate(d)
		idx := c.years[dn.Year()]
		i := idx.dayIndex(dn)
		acc.add(diffPrefix(idx.prefix[i+1], idx.prefix[i]))
	}
	return acc.result(len(dates), detailed), nil
}
