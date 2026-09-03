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
// 语义；粗粒度即基本位投影（t.Coarse()）。合法细粒度值全集为
// {1, 2, 6, 10, 17}，五值 MECE；零值 DayTypeUnknown(0) 表示未定义类型
// （非合法取值），可直接作为默认值。
type DayType uint8

const (
	// DayTypeUnknown 未定义类型（零值/默认值）：非合法细粒度取值，
	// 判类恒 false，String() 输出 "unknown"。
	DayTypeUnknown DayType = 0
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

// dayTypeNames 位名称（type_label 的分段名），按位常量显式索引：
// 位序或常量名调整时此处随编译检查自动跟随，无下标隐式耦合；
// 分段名与常量名逐字对应（去 DayType 前缀的小写蛇形）：
// Work→"work"、Rest→"rest"、Festival→"festival"、
// AdjustedRest→"adjusted_rest"、AdjustedWork→"adjusted_work"。
var dayTypeNames = [...]string{
	DayTypeWork:         "work",
	DayTypeRest:         "rest",
	DayTypeFestival:     "festival",
	DayTypeAdjustedRest: "adjusted_rest",
	DayTypeAdjustedWork: "adjusted_work",
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
	return t.IsValid() && t.Coarse() == DayTypeWork
}

// IsRest 报告该日是否为放假日：合法值且基本位投影等于 DayTypeRest。
// 任何非法值均返回 false。
func (t DayType) IsRest() bool {
	return t.IsValid() && t.Coarse() == DayTypeRest
}

// IsFestivalRest 报告该日是否为节日放假日：合法值且调整位投影等于
// DayTypeFestival（与 IsWork/IsRest 同构，先校验再投影判等）。
func (t DayType) IsFestivalRest() bool {
	return t.IsValid() && t.Adjustment() == DayTypeFestival
}

// IsAdjustedRestDay 报告该日是否为调休放假日：合法值且调整位投影等于
// DayTypeAdjustedRest；任何非法值均返回 false。
func (t DayType) IsAdjustedRestDay() bool {
	return t.IsValid() && t.Adjustment() == DayTypeAdjustedRest
}

// IsAdjustedWorkDay 报告该日是否为补班上班日：合法值且调整位投影等于
// DayTypeAdjustedWork；任何非法值均返回 false。
func (t DayType) IsAdjustedWorkDay() bool {
	return t.IsValid() && t.Adjustment() == DayTypeAdjustedWork
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

// joinNames 按位从低到高以 "|" 连接 t 含有的位名（dayTypeNames），
// 存在未知位时追加 "unknown"；不含任何已知位时返回 "unknown"。
// 仅 String() 的非法值路径使用。
func (t DayType) joinNames() string {
	var parts []string
	known := DayType(0)
	for bit, name := range dayTypeNames {
		if name == "" || t&DayType(bit) == 0 {
			continue
		}
		parts = append(parts, name)
		known |= DayType(bit)
	}
	if t&^known != 0 {
		parts = append(parts, "unknown")
	}
	if len(parts) == 0 {
		return "unknown"
	}
	return strings.Join(parts, "|")
}

// dayTypeStrings 初始化期预计算的字符串表（覆盖全部 5 位组合域 0~31，
// 合法值 {0, 1, 2, 6, 10, 17} 随之物化其中），由 dayTypeNames 一次性
// 构建而来——分段名单一来源，重构自动生效；String() 对表内值直接
// 查表返回，无逐次运行时构建。
var dayTypeStrings = func() [32]string {
	var s [32]string
	for v := range 32 {
		s[v] = DayType(v).joinNames()
	}
	return s
}()

// String 返回 DayType 的字符串表示：5 位组合域内的值（含合法五值与
// 零值 Unknown）直接查预计算表 dayTypeStrings，如 "unknown"、"work"、
// "rest|festival"、"work|adjusted_work"；域外值（含未定义位 ≥32）运行时
// 按 joinNames 逐位构建——已知位从低到高以 "|" 连接，存在未知位时
// 追加 "unknown"。
func (t DayType) String() string {
	if t < DayType(len(dayTypeStrings)) {
		return dayTypeStrings[t]
	}
	return t.joinNames()
}
