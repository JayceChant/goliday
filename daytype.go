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
//	DayTypeFestival   1<<2 过节：法定节日当天（放假），当日新增法定假期
//	DayTypeAdjusted   1<<3 调休：原工作日被调整为休息（非节日当天），不新增假期
//	DayTypeCompensate 1<<4 补班：原周末被调整为上班
//
// 全部组合值的按位或均为 all-of（合取）语义，不存在 any-of（并集物化值）
// 语义；粗粒度即基本位投影（t & (Work|Rest)）。合法细粒度值全集为
// {1, 2, 6, 10, 17}，五值 MECE。
type DayType uint8

const (
	// DayTypeWork 上班（粗粒度基本位；单值即普通工作日）。
	DayTypeWork DayType = 1 << 0
	// DayTypeRest 放假（粗粒度基本位；单值即普通周休，未调整时必然为周末）。
	DayTypeRest DayType = 1 << 1
	// DayTypeFestival 过节：法定节日当天（放假），当日新增法定假期。
	DayTypeFestival DayType = 1 << 2
	// DayTypeAdjusted 调休：原工作日被调整为休息（非节日当天），不新增假期。
	DayTypeAdjusted DayType = 1 << 3
	// DayTypeCompensate 补班：原周末被调整为上班。
	DayTypeCompensate DayType = 1 << 4
)

// dayTypeNames 位名称，按位从低到高排列。
var dayTypeNames = [...]string{
	"work",       // 1<<0
	"rest",       // 1<<1
	"festival",   // 1<<2
	"adjusted",   // 1<<3
	"compensate", // 1<<4
}

// Coarse 返回该日期类型的粗粒度投影（基本位掩码），
// 即 t & (DayTypeWork|DayTypeRest)；合法值上结果 ∈ {1, 2} 且幂等。
// 归属判定无需优先级消歧：t & DayTypeRest != 0 → 放假，
// t & DayTypeWork != 0 → 上班，合法值恰一非零。
func (t DayType) Coarse() DayType {
	return t & (DayTypeWork | DayTypeRest)
}

// IsWorkday 报告该日是否为上班日（t 含 DayTypeWork 位）。
func (t DayType) IsWorkday() bool { return t&DayTypeWork != 0 }

// IsHoliday 报告该日是否为放假日（t 含 DayTypeRest 位）。
func (t DayType) IsHoliday() bool { return t&DayTypeRest != 0 }

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
