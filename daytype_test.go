// 黑盒测试（package goliday_test）：仅经导出 API 验证 DayType 枚举契约
// 与全 256 取值的不变量，不触及实现细节。
package goliday_test

import (
	"strings"
	"testing"

	"goliday"
)

// validFineCombos 细粒度合法组合全集（6 种）及其期望数值与字符串。
var validFineCombos = []struct {
	name string
	dt   goliday.DayType
	val  goliday.DayType
	str  string
}{
	{"Ordinary", goliday.DayTypeOrdinary, 1, "ordinary"},
	{"Weekend", goliday.DayTypeWeekend, 4, "weekend"},
	{"Compensate|Weekend", goliday.DayTypeCompensate | goliday.DayTypeWeekend, 6, "compensate|weekend"},
	{"Festival|Weekend", goliday.DayTypeFestival | goliday.DayTypeWeekend, 12, "weekend|festival"},
	{"Adjusted", goliday.DayTypeAdjusted, 16, "adjusted"},
	{"Festival|Adjusted", goliday.DayTypeFestival | goliday.DayTypeAdjusted, 24, "festival|adjusted"},
}

// legalCombos 细粒度合法组合全集（数值集合），供穷举与 Calendar 校验复用。
var legalCombos = map[goliday.DayType]bool{
	1: true, 4: true, 6: true, 12: true, 16: true, 24: true,
}

// TestFineGrainedValues 细粒度合法组合全集的数值断言。
func TestFineGrainedValues(t *testing.T) {
	for _, tt := range validFineCombos {
		if tt.dt != tt.val {
			t.Errorf("%s = %d, want %d", tt.name, tt.dt, tt.val)
		}
	}
	if goliday.DayTypeWorkday != 3 {
		t.Errorf("DayTypeWorkday = %d, want 3", goliday.DayTypeWorkday)
	}
	if goliday.DayTypeHoliday != 28 {
		t.Errorf("DayTypeHoliday = %d, want 28", goliday.DayTypeHoliday)
	}
}

// TestCoarse Coarse() 的粗粒度映射断言。
func TestCoarse(t *testing.T) {
	tests := []struct {
		name string
		dt   goliday.DayType
		want goliday.DayType
	}{
		{"Ordinary", goliday.DayTypeOrdinary, goliday.DayTypeWorkday},
		{"Compensate|Weekend", goliday.DayTypeCompensate | goliday.DayTypeWeekend, goliday.DayTypeWorkday},
		{"Weekend", goliday.DayTypeWeekend, goliday.DayTypeHoliday},
		{"Festival|Weekend", goliday.DayTypeFestival | goliday.DayTypeWeekend, goliday.DayTypeHoliday},
		{"Adjusted", goliday.DayTypeAdjusted, goliday.DayTypeHoliday},
		{"Festival|Adjusted", goliday.DayTypeFestival | goliday.DayTypeAdjusted, goliday.DayTypeHoliday},
	}
	for _, tt := range tests {
		if got := tt.dt.Coarse(); got != tt.want {
			t.Errorf("%s.Coarse() = %d, want %d", tt.name, got, tt.want)
		}
	}
}

// TestIsWorkdayIsHoliday IsWorkday/IsHoliday 与 Coarse 一致性断言。
func TestIsWorkdayIsHoliday(t *testing.T) {
	all := []struct {
		name    string
		dt      goliday.DayType
		workday bool
		holiday bool
	}{
		{"Workday(粗粒度)", goliday.DayTypeWorkday, true, false},
		{"Holiday(粗粒度)", goliday.DayTypeHoliday, false, true},
		{"Ordinary", goliday.DayTypeOrdinary, true, false},
		{"Weekend", goliday.DayTypeWeekend, false, true},
		{"Compensate|Weekend", goliday.DayTypeCompensate | goliday.DayTypeWeekend, true, false},
		{"Festival|Weekend", goliday.DayTypeFestival | goliday.DayTypeWeekend, false, true},
		{"Adjusted", goliday.DayTypeAdjusted, false, true},
		{"Festival|Adjusted", goliday.DayTypeFestival | goliday.DayTypeAdjusted, false, true},
	}
	for _, tt := range all {
		if wantWorkday := tt.dt.Coarse() == goliday.DayTypeWorkday; tt.dt.IsWorkday() != wantWorkday {
			t.Errorf("%s.IsWorkday() = %v, want %v", tt.name, tt.dt.IsWorkday(), wantWorkday)
		}
		if tt.dt.IsWorkday() != tt.workday {
			t.Errorf("%s.IsWorkday() = %v, want %v", tt.name, tt.dt.IsWorkday(), tt.workday)
		}
		if wantHoliday := tt.dt.Coarse() == goliday.DayTypeHoliday; tt.dt.IsHoliday() != wantHoliday {
			t.Errorf("%s.IsHoliday() = %v, want %v", tt.name, tt.dt.IsHoliday(), wantHoliday)
		}
		if tt.dt.IsHoliday() != tt.holiday {
			t.Errorf("%s.IsHoliday() = %v, want %v", tt.name, tt.dt.IsHoliday(), tt.holiday)
		}
	}
}

// TestString String() 的字符串断言（含粗粒度值）。
func TestString(t *testing.T) {
	for _, tt := range validFineCombos {
		if got := tt.dt.String(); got != tt.str {
			t.Errorf("%s.String() = %q, want %q", tt.name, got, tt.str)
		}
	}
	if got := goliday.DayTypeWorkday.String(); got != "workday" {
		t.Errorf("DayTypeWorkday.String() = %q, want %q", got, "workday")
	}
	if got := goliday.DayTypeHoliday.String(); got != "holiday" {
		t.Errorf("DayTypeHoliday.String() = %q, want %q", got, "holiday")
	}
}

// TestDayTypeExhaustiveInvariants 穷举 uint8 全部 256 个取值，验证
// Coarse 映射封闭且幂等、IsWorkday/IsHoliday 恰一为真、String 输出的
// 每个分段均为合法名称（细粒度 5 名 + 粗粒度 2 名 + unknown）。
func TestDayTypeExhaustiveInvariants(t *testing.T) {
	legalNames := map[string]bool{
		"ordinary": true, "compensate": true, "weekend": true,
		"festival": true, "adjusted": true,
		"workday": true, "holiday": true, "unknown": true,
	}
	for v := 0; v <= 255; v++ {
		dt := goliday.DayType(v)
		t.Run(dt.String(), func(t *testing.T) {
			coarse := dt.Coarse()
			if coarse != goliday.DayTypeWorkday && coarse != goliday.DayTypeHoliday {
				t.Fatalf("DayType(%d).Coarse() = %d，必须 ∈ {Workday, Holiday}", v, coarse)
			}
			if coarse.Coarse() != coarse {
				t.Fatalf("Coarse 不幂等: %d → %d → %d", v, coarse, coarse.Coarse())
			}
			if dt.IsWorkday() == dt.IsHoliday() {
				t.Fatalf("DayType(%d) IsWorkday/IsHoliday 必须恰一为真", v)
			}
			for _, part := range strings.Split(dt.String(), "|") {
				if !legalNames[part] {
					t.Fatalf("DayType(%d).String() = %q 含非法分段 %q", v, dt.String(), part)
				}
			}
			// 合法组合之外不应出现 IsWorkday 分支歧义：粗值自身合法。
			if v == int(goliday.DayTypeWorkday) || v == int(goliday.DayTypeHoliday) {
				if dt.String() != "workday" && dt.String() != "holiday" {
					t.Fatalf("粗粒度值 %d 的 String = %q", v, dt.String())
				}
			}
		})
	}
}
