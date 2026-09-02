// Package goliday 提供中国法定节假日与调休工作日的日期类型判定能力。
package goliday

import "strings"

// DayType 表示某一天的日期类型，采用位掩码（bitmask）编码，
// 分为「终态双层」：
//
// 粗粒度基本位（互斥，恰一个，表达当日最终是否上班）：
//
//	DayTypeWork 1<<0 上班；单值 1 即普通工作日
//	DayTypeRest 1<<1 放假；单值 2 即普通周休（未调整时必然为周末）
//
// 调整位（互斥，至多一个，依附于基本位）：
//
//	DayTypeFestival     1<<2 过节：法定节日当天（放假），当日新增法定假期
//	DayTypeAdjustedRest 1<<3 调休：原工作日被调整为休息（非节日当天），不新增假期
//	DayTypeAdjustedWork 1<<4 补班：原周末被调整为上班
//
// 全部组合值的按位或均为 all-of（合取）语义，不存在 any-of（并集物化值）
// 语义；粗粒度即基本位投影（t & dayTypeCoarseMask）。合法细粒度值全集为
// {1, 2, 6, 10, 17}，五值 MECE。
type DayType uint8

const (
	// DayTypeWork 上班（粗粒度基本位；单值即普通工作日）。
	DayTypeWork DayType = 1 << 0
	// DayTypeRest 放假（粗粒度基本位；单值即普通周休，未调整时必然为周末）。
	DayTypeRest DayType = 1 << 1
	// DayTypeFestival 过节：法定节日当天（放假），当日新增法定假期。
	DayTypeFestival DayType = 1 << 2
	// DayTypeAdjustedRest 调休：原工作日被调整为休息（非节日当天），不新增假期。
	DayTypeAdjustedRest DayType = 1 << 3
	// DayTypeAdjustedWork 补班：原周末被调整为上班。
	DayTypeAdjustedWork DayType = 1 << 4
)

// 合法细粒度组合值（终态五值，MECE），由基本位与调整位组合而成。
const (
	// DayTypeFestivalRest 节日放假日：节日当天（无论落在工作日还是周末）。
	DayTypeFestivalRest = DayTypeRest | DayTypeFestival
	// DayTypeAdjustedRestDay 调休放假日：原工作日被调整为休息（非节日当天）。
	DayTypeAdjustedRestDay = DayTypeRest | DayTypeAdjustedRest
	// DayTypeAdjustedWorkDay 补班上班日：原周末被调整为上班。
	DayTypeAdjustedWorkDay = DayTypeWork | DayTypeAdjustedWork
)

// dayTypeCoarseMask 粗粒度掩码：两个基本位之并（Work|Rest = 3），
// 仅供内部投影/判类使用（Coarse/IsWork/IsRest），本身不是合法的
// DayType 取值（单值 3 为非法值）。
const dayTypeCoarseMask = DayTypeWork | DayTypeRest

// dayTypeAdjustMask 调整位掩码：三个调整位之并
// （Festival|AdjustedRest|AdjustedWork = 28），仅供内部判类使用
// （IsFestivalRest/IsAdjustedRestDay/IsAdjustedWorkDay），本身不是
// 合法的 DayType 取值（多调整位并存为非法值）。
const dayTypeAdjustMask = DayTypeFestival | DayTypeAdjustedRest | DayTypeAdjustedWork

// dayTypeNames 位名称（type_label 的分段名），按位从低到高排列；
// 调整位取其动宾语义的简写（adjusted rest → "adjusted"、
// adjusted work → "compensate"，沿用补班惯用词根），与常量名不必逐字一致。
var dayTypeNames = [...]string{
	"work",       // 1<<0
	"rest",       // 1<<1
	"festival",   // 1<<2
	"adjusted",   // 1<<3
	"compensate", // 1<<4
}

// validFineValues 细粒度合法值全集（五值 MECE）。
var validFineValues = [...]DayType{
	DayTypeWork,
	DayTypeRest,
	DayTypeFestivalRest,
	DayTypeAdjustedRestDay,
	DayTypeAdjustedWorkDay,
}

// Coarse 返回该日期类型的粗粒度投影（基本位掩码），
// 即 t & dayTypeCoarseMask；合法值上结果 ∈ {1, 2} 且幂等，
// 非法值（如 3，同含两个基本位）返回原值本身。
func (t DayType) Coarse() DayType {
	return t & dayTypeCoarseMask
}

// Adjustment 返回该日期类型的调整位投影（调整掩码），
// 即 t & dayTypeAdjustMask，与 Coarse 相对应；合法值上
// 结果 ∈ {0, 4, 8, 16}（无调整位时为 0）且幂等，
// 非法值（如 12，多调整位并存）返回原值本身。
func (t DayType) Adjustment() DayType {
	return t & dayTypeAdjustMask
}

// IsWork 报告该日是否为上班日：合法值且基本位投影等于 DayTypeWork。
// 任何非法值（3 同含两基本位、5/9 过节调休配上班位等）均返回 false。
func (t DayType) IsWork() bool {
	return t.IsValid() && t&dayTypeCoarseMask == DayTypeWork
}

// IsRest 报告该日是否为放假日：合法值且基本位投影等于 DayTypeRest。
// 任何非法值均返回 false。
func (t DayType) IsRest() bool {
	return t.IsValid() && t&dayTypeCoarseMask == DayTypeRest
}

// IsFestivalRest 报告该日是否为节日放假日：合法值且调整位投影等于
// DayTypeFestival（与 IsWork/IsRest 同构，先校验再投影判等）。
func (t DayType) IsFestivalRest() bool {
	return t.IsValid() && t&dayTypeAdjustMask == DayTypeFestival
}

// IsAdjustedRestDay 报告该日是否为调休放假日：合法值且调整位投影等于
// DayTypeAdjustedRest；任何非法值均返回 false。
func (t DayType) IsAdjustedRestDay() bool {
	return t.IsValid() && t&dayTypeAdjustMask == DayTypeAdjustedRest
}

// IsAdjustedWorkDay 报告该日是否为补班上班日：合法值且调整位投影等于
// DayTypeAdjustedWork；任何非法值均返回 false。
func (t DayType) IsAdjustedWorkDay() bool {
	return t.IsValid() && t&dayTypeAdjustMask == DayTypeAdjustedWork
}

// IsValid 报告 t 是否为合法细粒度值（{1, 2, 6, 10, 17} 之一）。
func (t DayType) IsValid() bool {
	for _, v := range validFineValues {
		if t == v {
			return true
		}
	}
	return false
}

// String 返回 DayType 的字符串表示：按位从低到高以 "|" 连接小写位名，
// 如 "work"、"rest"、"rest|festival"、"rest|adjusted"、"work|compensate"
// （粗粒度值 1/2 天然输出 "work"/"rest"，无需特判）；
// 存在未知位时追加 "unknown"；空值（0）返回 "unknown"。
func (t DayType) String() string {
	var parts []string
	known := DayType(0)
	for i, name := range dayTypeNames {
		bit := DayType(1) << i
		if t&bit != 0 {
			parts = append(parts, name)
			known |= bit
		}
	}
	if t&^known != 0 {
		parts = append(parts, "unknown")
	}
	if len(parts) == 0 {
		return "unknown"
	}
	return strings.Join(parts, "|")
}
