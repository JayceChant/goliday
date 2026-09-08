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

// normalizeDate 将任意时刻规范化为其所在日的整数日期键：year 为该日
// 所在年份，day 为距当年元旦的天数下标（0-based，闰年至多 365）。壁钟
// 语义与时刻/时区解耦：按 t 自身时区取年与年内天序（元旦当天为 0）。
func normalizeDate(t time.Time) (year, day int) {
	return t.Year(), t.YearDay() - 1
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

// yearIndex 单一年份配置的快速查找索引与统计前缀和。
type yearIndex struct {
	// adjust 被调整日期的稀疏终态表：键为距元旦的天数下标（0-based），
	// 值为该日细粒度终态类型。配置 Validate 保证 off∩work、work∩festival
	// 互斥，仅工作日节日可同时在 off 与 festival 中，故构建时按 off →
	// work → festival 顺序覆写（festival 最后），单表即为带优先级的终态。
	adjust map[int]DayType

	// first 该年元旦（UTC 午夜，用于周休判定与日期重建）；days 该年
	// 天数（365/366，前缀和的有效长度）。
	first time.Time
	days  int
	// prefix 细粒度组合计数前缀和，定长 [366]（覆盖闰年最大天数，平年
	// 尾部 1 个元素闲置），前 days 个元素有效（无全零首元素）：
	// prefix[i] 为闭区间 [元旦, 元旦+i天]（含两端）的组合累计。
	// 左闭右开查询 [a, b) 须转换下标后差分：prefix[idx(b)-1] -
	// prefix[idx(a)-1]，下标为 -1（端点为元旦或之前）时以全零参与差分。
	// 构建完成后只读，可被多个 goroutine 并发访问。
	prefix [366]comboCounts
}

// dayType 返回该年某日（day 为距元旦的天数下标）的细粒度类型。
//
// 判断优先级：adjust 命中 → 该日被调整的终态（festival > work > off 已
// 在构建期按序覆写收敛为单一终态）；未命中回退周休判断（周六/周日 →
// Rest，否则 Work）。
func (idx *yearIndex) dayType(day int) DayType {
	if t, ok := idx.adjust[day]; ok {
		return t
	}
	if wd := idx.first.AddDate(0, 0, day).Weekday(); wd == time.Saturday || wd == time.Sunday {
		return DayTypeRest
	}
	return DayTypeWork
}

// cumulationAt 返回闭区间 [元旦, 元旦+i天] 的组合累计；i < 0（元旦之前，
// 无覆盖日）返回全零。闭区间前缀无全零首元素，左闭右开查询经此统一转换。
func (idx *yearIndex) cumulationAt(i int) comboCounts {
	if i < 0 {
		return comboCounts{}
	}
	return idx.prefix[i]
}

// buildPrefix 逐日判定并构建该年的组合计数前缀和（闭区间下标，仅写
// 前 days 个元素；尾部闲置元素保持零值，不参与差分）。
func (idx *yearIndex) buildPrefix() {
	for i := range idx.days {
		c := idx.cumulationAt(i - 1)
		c[comboIndex(idx.dayType(i))]++
		idx.prefix[i] = c
	}
}

// yearDays 返回指定年份的天数（365/366）：公历闰年直判（被 4 整除且
// 不被 100 整除，或被 400 整除）。Go time 包为外推公历，除闰年规则外
// 无其他日期调整，直判与「元旦至次年元旦差值」恒等价（宽年份区间
// 回归断言见 calendar_internal_test.go）。
func yearDays(year int) int {
	if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
		return 366
	}
	return 365
}

// Calendar 提供日期类型查询与统计能力。由 Store 一次性构建各年份的
// 查找索引与组合计数前缀和，构造完成后只读，可被多个 goroutine 并发调用。
type Calendar struct {
	years map[int]*yearIndex
}

// NewCalendar 将 Store 中各年份的稀疏配置烘焙为各年的调整终态稀疏表
// （键为年内天序），并为每年构建细粒度组合计数前缀和（供 StatsRange/
// Stats 差分统计）。
func NewCalendar(s *Store) *Calendar {
	c := &Calendar{years: make(map[int]*yearIndex)}
	for _, y := range s.Years() {
		cfg := s.Get(y)
		if cfg == nil {
			continue
		}
		yearIdx := &yearIndex{
			adjust: make(map[int]DayType, len(cfg.Adjust.Off)+len(cfg.Adjust.Work)+len(cfg.Festivals)),
			first:  time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC),
			days:   yearDays(y),
		}
		// 按 off → work → festival 顺序覆写（festival 最后），与判定
		// 优先级一致：工作日节日同时落在 off 与 festival 中，终态收敛
		// 为 FestivalRest；work 与其余表互斥（Validate 保证），覆写无歧义。
		for _, d := range cfg.Adjust.Off {
			_, day := normalizeDate(d)
			yearIdx.adjust[day] = DayTypeAdjustedRestDay
		}
		for _, d := range cfg.Adjust.Work {
			_, day := normalizeDate(d)
			yearIdx.adjust[day] = DayTypeAdjustedWorkDay
		}
		for _, f := range cfg.Festivals {
			_, day := normalizeDate(f.Date)
			yearIdx.adjust[day] = DayTypeFestivalRest
		}
		yearIdx.buildPrefix()
		c.years[y] = yearIdx
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

// coveredYears 返回左闭右开区间 [sYear/sDay, eYear/eDay)（已规范化，e
// 晚于 s）覆盖的全部年份（升序，含仅被部分覆盖的中间整年）。起点日序
// 不影响覆盖年份集合（起点必落在 sYear 年内），故不接收 sDay。
func coveredYears(sYear, eYear, eDay int) []int {
	last := eYear
	if eDay == 0 { // e 为某年元旦，末覆盖日属上一年
		last--
	}
	years := make([]int, 0, last-sYear+1)
	for y := sYear; y <= last; y++ {
		years = append(years, y)
	}
	return years
}

// Query 返回 date 的细粒度日期类型。任意时刻均先按其所在日规范化再查询。
//
// 判断优先级：节日当天 → FestivalRest（必为放假日）；work 命中 →
// AdjustedWorkDay；off 命中 → AdjustedRestDay；周休回退（周末 → Rest，
// 否则 Work）——前三级已构建期收敛为单一 adjust 表，查询一次命中。
// 该年未加载配置时返回包装 ErrYearNotLoaded 的错误，不再回退周休判断。
func (c *Calendar) Query(date time.Time) (DayType, error) {
	y, day := normalizeDate(date)
	idx := c.years[y]
	if idx == nil {
		return 0, fmt.Errorf("%w: %d", ErrYearNotLoaded, y)
	}
	return idx.dayType(day), nil
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
	sy, sd := normalizeDate(start)
	ey, ed := normalizeDate(end)
	if ey < sy || (ey == sy && ed <= sd) {
		return []Dated{}, nil
	}
	if missing := c.missingYears(coveredYears(sy, ey, ed)); missing != nil {
		return nil, yearNotLoadedErr(missing)
	}
	// 日期键重建为 UTC 午夜（time.Date 自动归一化天序溢出），供逐日
	// 输出与容量预估。
	s := time.Date(sy, 1, 1+sd, 0, 0, 0, 0, time.UTC)
	e := time.Date(ey, 1, 1+ed, 0, 0, 0, 0, time.UTC)
	out := make([]Dated, 0, int(e.Sub(s).Hours()/24)+1)
	for d := s; d.Before(e); d = d.AddDate(0, 0, 1) {
		y, day := normalizeDate(d)
		out = append(out, Dated{Date: d, Type: c.years[y].dayType(day)})
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
	sy, sd := normalizeDate(start)
	ey, ed := normalizeDate(end)
	if ey < sy || (ey == sy && ed <= sd) {
		return StatsResult{Coarse: map[DayType]int{}}, nil
	}
	if missing := c.missingYears(coveredYears(sy, ey, ed)); missing != nil {
		return StatsResult{}, yearNotLoadedErr(missing)
	}

	// 以 (年, 年内天序) 整数对逐段推进：段为年内左闭右开 [d, segEnd)，
	// segEnd 取该年天数（段延伸至次年元旦）或终点天序 ed；差分用闭
	// 区间下标（两端减一，负值由 cumulationAt 归零）。ed 为 0（end
	// 恰为次年元旦）时折叠为上一年末（天序 = 该年天数），避免进入
	// 未加载的终点年。
	if ed == 0 {
		ey--
		ed = c.years[ey].days
	}
	var acc comboTotals
	total := 0
	y, d := sy, sd
	for {
		idx := c.years[y]
		segEnd := idx.days
		if y == ey {
			segEnd = ed
		}
		acc.add(diffPrefix(idx.cumulationAt(segEnd-1), idx.cumulationAt(d-1)))
		total += segEnd - d
		if y == ey {
			break
		}
		d = 0
		y++
	}
	return acc.result(total, detailed), nil
}

// Stats 统计日期集合。dates 视为已去重升序的日期集合（去重与排序由
// 调用方负责），逐元素计数不去重。
//
// 实现上逐日取前缀和的单日差分（prefix[day] - prefix[day-1]），复用与
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
		y, _ := normalizeDate(d)
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
		y, day := normalizeDate(d)
		idx := c.years[y]
		acc.add(diffPrefix(idx.cumulationAt(day), idx.cumulationAt(day-1)))
	}
	return acc.result(len(dates), detailed), nil
}
