// Package goliday 提供中国法定节假日与调休工作日的日期类型判定能力。
package goliday

import "strings"

// DayType 表示某一天的日期类型，采用位掩码（bitmask）编码。
//
// 细粒度语义位（低 5 位，可组合）：
//
//	DayTypeOrdinary   1<<0 普通工作日：常规上班日，不涉及调休（工作日段）
//	DayTypeCompensate 1<<1 补班：因节假日调休而需要上班的日子（工作日段）
//	DayTypeWeekend    1<<2 周末：周六、周日（节假日段）
//	DayTypeFestival   1<<3 节日：法定节假日当天（节假日段）
//	DayTypeAdjusted   1<<4 调休：为凑长假而调整休息的日子（节假日段）
//
// 粗粒度汇总值（两个互斥的集合值）：
//
//	DayTypeWorkday = DayTypeOrdinary | DayTypeCompensate                  // 上班日 = 3
//	DayTypeHoliday = DayTypeWeekend | DayTypeFestival | DayTypeAdjusted   // 休息日 = 28
//
// 粗细粒度的映射规则见 Coarse 方法。
type DayType uint8

const (
	// DayTypeOrdinary 普通工作日（细粒度，工作日段）。
	DayTypeOrdinary DayType = 1 << 0
	// DayTypeCompensate 补班日：调休产生的上班日（细粒度，工作日段）。
	DayTypeCompensate DayType = 1 << 1
	// DayTypeWeekend 周末（细粒度，节假日段）。
	DayTypeWeekend DayType = 1 << 2
	// DayTypeFestival 法定节日（细粒度，节假日段）。
	DayTypeFestival DayType = 1 << 3
	// DayTypeAdjusted 调休休息日（细粒度，节假日段）。
	DayTypeAdjusted DayType = 1 << 4

	// DayTypeWorkday 粗粒度：上班日（普通工作日 | 补班）。
	DayTypeWorkday = DayTypeOrdinary | DayTypeCompensate
	// DayTypeHoliday 粗粒度：休息日（周末 | 节日 | 调休）。
	DayTypeHoliday = DayTypeWeekend | DayTypeFestival | DayTypeAdjusted
)

// dayTypeNames 细粒度位名称，按位从低到高排列。
var dayTypeNames = [...]string{
	"ordinary",   // 1<<0
	"compensate", // 1<<1
	"weekend",    // 1<<2
	"festival",   // 1<<3
	"adjusted",   // 1<<4
}

// Coarse 返回该日期类型对应的粗粒度类型，映射规则（按优先级）：
//
//  1. 含 DayTypeCompensate（补班）位 → DayTypeWorkday，补班优先归为上班日；
//  2. 否则含 DayTypeWeekend/DayTypeFestival/DayTypeAdjusted 任一位 → DayTypeHoliday；
//  3. 否则（仅 DayTypeOrdinary、空值或未知位）→ DayTypeWorkday。
func (t DayType) Coarse() DayType {
	if t&DayTypeCompensate != 0 {
		return DayTypeWorkday
	}
	if t&(DayTypeWeekend|DayTypeFestival|DayTypeAdjusted) != 0 {
		return DayTypeHoliday
	}
	return DayTypeWorkday
}

// IsWorkday 报告该日是否为上班日（等价于 Coarse() == DayTypeWorkday）。
func (t DayType) IsWorkday() bool { return t.Coarse() == DayTypeWorkday }

// IsHoliday 报告该日是否为休息日（等价于 Coarse() == DayTypeHoliday）。
func (t DayType) IsHoliday() bool { return t.Coarse() == DayTypeHoliday }

// String 返回 DayType 的字符串表示：
//
//   - 粗粒度值本身输出 "workday" / "holiday"；
//   - 其余值按细粒度位从低到高以 "|" 连接小写名称，
//     如 "festival|adjusted"、"compensate|weekend"、"weekend"；
//   - 存在未知位时追加 "unknown"；空值（0）返回 "unknown"。
func (t DayType) String() string {
	switch t {
	case DayTypeWorkday:
		return "workday"
	case DayTypeHoliday:
		return "holiday"
	}
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
