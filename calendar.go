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

// 细粒度合法值在 comboCounts 中的固定下标（值见 comboValues）。
const (
	comboOrdinary   = iota // 1  Work
	comboWeekend           // 2  Rest
	comboFestival          // 6  FestivalRest
	comboAdjusted          // 10 AdjustedRestDay
	comboCompensate        // 17 AdjustedWorkDay
)

// comboValues 与 comboCounts 下标一一对应的合法细粒度值，升序。
var comboValues = [...]DayType{
	DayTypeWork,
	DayTypeRest,
	DayTypeFestivalRest,
	DayTypeAdjustedRestDay,
	DayTypeAdjustedWorkDay,
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

// comboCounts 5 种合法细粒度值各自的年内累计天数（按值计数，五值 MECE）。
// 粗粒度计数可由其线性组合导出。单一类型年内天数有界（普通工作日至多
// 248 天，其余类型更少），uint8 足以存储；跨年/多段累加不得直接在本类型
// 上进行（会溢出），须先转入 comboTotals。
type comboCounts [len(comboValues)]uint8

// diff 返回两组年内前缀计数的差（b-a，前缀和单调不减于被减数，无下溢）。
func diffPrefix(b, a comboCounts) comboCounts {
	var out comboCounts
	for i := range b {
		out[i] = b[i] - a[i]
	}
	return out
}

// comboTotals 跨年统计的宽类型累加器：各段年内差分（comboCounts，uint8）
// 显式转入 int 后再相加，避免 uint8 多段直接累加溢出。
type comboTotals [len(comboValues)]int

// add 累加一段年内差分计数（逐元素转 int）。
func (c *comboTotals) add(o comboCounts) {
	for i, v := range o {
		c[i] += int(v)
	}
}

// result 将跨年累计计数导出为统计结果：细粒度五键 MECE（之和恒等于
// total），粗粒度按基本位投影归并。detailed=false 时 Fine 为 nil。
func (c comboTotals) result(total int, detailed bool) StatsResult {
	r := StatsResult{
		Total: total,
		Coarse: map[DayType]int{
			DayTypeWork: c[comboOrdinary] + c[comboCompensate],
			DayTypeRest: c[comboWeekend] + c[comboFestival] + c[comboAdjusted],
		},
	}
	if detailed {
		r.Fine = map[DayType]int{
			DayTypeWork:         c[comboOrdinary],
			DayTypeRest:         c[comboWeekend],
			DayTypeFestival:     c[comboFestival],
			DayTypeAdjustedRest: c[comboAdjusted],
			DayTypeAdjustedWork: c[comboCompensate],
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
	// prefix 细粒度组合计数前缀和，长度 days（与年天数一致，无全零首
	// 元素）：prefix[i] 为闭区间 [元旦, 元旦+i天]（含两端）的组合累计。
	// 左闭右开查询 [a, b) 须转换下标后差分：prefix[idx(b)-1] -
	// prefix[idx(a)-1]，下标为 -1（端点为元旦或之前）时以全零参与差分。
	// 构建完成后只读，可被多个 goroutine 并发访问。
	prefix []comboCounts
}

// dayType 返回该年某日（已规范化）的细粒度类型。
//
// 判断优先级：节日当天 → FestivalRest（必为放假日）；work 命中 →
// AdjustedWorkDay（补班）；off 命中 → AdjustedRestDay（调休）；周休回退
// （周六/周日 → Rest，否则 Work）。
func (idx *yearIndex) dayType(d time.Time) DayType {
	if _, ok := idx.festival[d]; ok {
		return DayTypeFestivalRest
	}
	if _, ok := idx.work[d]; ok {
		return DayTypeAdjustedWorkDay
	}
	if _, ok := idx.off[d]; ok {
		return DayTypeAdjustedRestDay
	}
	if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return DayTypeRest
	}
	return DayTypeWork
}

// dayIndex 返回该年某日（已规范化）距元旦的天数下标（0-based）。
func (idx *yearIndex) dayIndex(d time.Time) int {
	return int(d.Sub(idx.first).Hours() / 24)
}

// nextYearStart 返回该年次年的元旦（UTC 午夜）。
func (idx *yearIndex) nextYearStart() time.Time {
	return idx.first.AddDate(1, 0, 0)
}

// cumAt 返回闭区间 [元旦, 元旦+i天] 的组合累计；i < 0（元旦之前，无
// 覆盖日）返回全零。闭区间前缀无全零首元素，左闭右开查询经此统一转换。
func (idx *yearIndex) cumAt(i int) comboCounts {
	if i < 0 {
		return comboCounts{}
	}
	return idx.prefix[i]
}

// comboRange 返回年内左闭右开区间 [a, b) 的组合计数（b 可为次年元旦）。
// 闭区间下标转换：两端取 dayIndex-1，负值由 cumAt 归零。
func (idx *yearIndex) comboRange(a, b time.Time) comboCounts {
	return diffPrefix(idx.cumAt(idx.dayIndex(b)-1), idx.cumAt(idx.dayIndex(a)-1))
}

// buildPrefix 逐日判定并构建该年的组合计数前缀和（闭区间下标）。
func (idx *yearIndex) buildPrefix() {
	idx.prefix = make([]comboCounts, idx.days)
	for i := range idx.prefix {
		c := idx.cumAt(i - 1)
		c[comboIndex(idx.dayType(idx.first.AddDate(0, 0, i)))]++
		idx.prefix[i] = c
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
// 判断优先级：节日当天 → FestivalRest（必为放假日）；work 命中 →
// AdjustedWorkDay；off 命中 → AdjustedRestDay；周休回退（周末 → Rest，
// 否则 Work）。该年未加载配置时返回包装 ErrYearNotLoaded 的错误，
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

// IsWork 报告 date 是否为上班日（与基本位 Work 对齐）。
func (c *Calendar) IsWork(date time.Time) (bool, error) {
	t, err := c.Query(date)
	if err != nil {
		return false, err
	}
	return t.IsWork(), nil
}

// IsRest 报告 date 是否为放假日（与基本位 Rest 对齐；含普通周休、
// 节日放假日与调休放假日，不含补班）。
func (c *Calendar) IsRest(date time.Time) (bool, error) {
	t, err := c.Query(date)
	if err != nil {
		return false, err
	}
	return t.IsRest(), nil
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
//	Coarse 粗粒度计数，键为 DayTypeWork / DayTypeRest；
//	Fine   细粒度五键 MECE 计数（普通工作日/普通周休/节日放假日/
//	       调休放假日/补班日），各键之和恒等于 Total；detailed=false 时为 nil。
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

	var acc comboTotals
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

	var acc comboTotals
	for _, d := range dates {
		dn := normalizeDate(d)
		idx := c.years[dn.Year()]
		i := idx.dayIndex(dn)
		acc.add(diffPrefix(idx.cumAt(i), idx.cumAt(i-1)))
	}
	return acc.result(len(dates), detailed), nil
}
