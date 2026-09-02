// 黑盒测试（package goliday_test）：仅经导出 API 验证 DayType 枚举契约
// 与全 256 取值的不变量，不触及实现细节。
package goliday_test

import (
	"strings"
	"testing"

	"github.com/JayceChant/goliday"
)

// validFineValues 细粒度合法值全集（5 种）及其期望数值与字符串。
var validFineValues = []struct {
	name string
	dt   goliday.DayType
	val  goliday.DayType
	str  string
}{
	{"Work", goliday.DayTypeWork, 1, "work"},
	{"Rest", goliday.DayTypeRest, 2, "rest"},
	{"FestivalRest", goliday.DayTypeFestivalRest, 6, "rest|festival"},
	{"AdjustedRestDay", goliday.DayTypeAdjustedRestDay, 10, "rest|adjusted"},
	{"AdjustedWorkDay", goliday.DayTypeAdjustedWorkDay, 17, "work|compensate"},
}

// legalFineValues 细粒度合法值全集（数值集合），供穷举与 Calendar 校验复用。
var legalFineValues = map[goliday.DayType]bool{
	1: true, 2: true, 6: true, 10: true, 17: true,
}

// TestFineGrainedValues 细粒度合法值全集的数值断言。
func TestFineGrainedValues(t *testing.T) {
	for _, tt := range validFineValues {
		if tt.dt != tt.val {
			t.Errorf("%s = %d, want %d", tt.name, tt.dt, tt.val)
		}
	}
	if goliday.DayTypeWork != 1 {
		t.Errorf("DayTypeWork = %d, want 1", goliday.DayTypeWork)
	}
	if goliday.DayTypeRest != 2 {
		t.Errorf("DayTypeRest = %d, want 2", goliday.DayTypeRest)
	}
	// 组合值常量 = 基本位 | 调整位。
	if want := goliday.DayTypeRest | goliday.DayTypeFestival; goliday.DayTypeFestivalRest != want {
		t.Errorf("DayTypeFestivalRest = %d, want %d", goliday.DayTypeFestivalRest, want)
	}
	if want := goliday.DayTypeRest | goliday.DayTypeAdjustedRest; goliday.DayTypeAdjustedRestDay != want {
		t.Errorf("DayTypeAdjustedRestDay = %d, want %d", goliday.DayTypeAdjustedRestDay, want)
	}
	if want := goliday.DayTypeWork | goliday.DayTypeAdjustedWork; goliday.DayTypeAdjustedWorkDay != want {
		t.Errorf("DayTypeAdjustedWorkDay = %d, want %d", goliday.DayTypeAdjustedWorkDay, want)
	}
}

// TestCoarse Coarse() 的粗粒度投影断言（基本位掩码）。
func TestCoarse(t *testing.T) {
	tests := []struct {
		name string
		dt   goliday.DayType
		want goliday.DayType
	}{
		{"Work", goliday.DayTypeWork, goliday.DayTypeWork},
		{"AdjustedWorkDay", goliday.DayTypeAdjustedWorkDay, goliday.DayTypeWork},
		{"Rest", goliday.DayTypeRest, goliday.DayTypeRest},
		{"FestivalRest", goliday.DayTypeFestivalRest, goliday.DayTypeRest},
		{"AdjustedRestDay", goliday.DayTypeAdjustedRestDay, goliday.DayTypeRest},
		{"非法值 3", 3, 3}, // 同含两基本位，投影保留原值
	}
	for _, tt := range tests {
		if got := tt.dt.Coarse(); got != tt.want {
			t.Errorf("%s.Coarse() = %d, want %d", tt.name, got, tt.want)
		}
	}
}

// TestIsWorkIsRest IsWork/IsRest 与粗粒度投影一致性断言；
// 非法值（如 3）两者均 false。
func TestIsWorkIsRest(t *testing.T) {
	all := []struct {
		name string
		dt   goliday.DayType
		work bool
		rest bool
	}{
		{"Work(粗粒度)", goliday.DayTypeWork, true, false},
		{"Rest(粗粒度)", goliday.DayTypeRest, false, true},
		{"AdjustedWorkDay", goliday.DayTypeAdjustedWorkDay, true, false},
		{"FestivalRest", goliday.DayTypeFestivalRest, false, true},
		{"AdjustedRestDay", goliday.DayTypeAdjustedRestDay, false, true},
		{"非法值 0", 0, false, false},
		{"非法值 3", 3, false, false},
		{"非法值 5", 5, false, false},
	}
	for _, tt := range all {
		if tt.dt.IsWork() != tt.work {
			t.Errorf("%s(%d).IsWork() = %v, want %v", tt.name, tt.dt, tt.dt.IsWork(), tt.work)
		}
		if tt.dt.IsRest() != tt.rest {
			t.Errorf("%s(%d).IsRest() = %v, want %v", tt.name, tt.dt, tt.dt.IsRest(), tt.rest)
		}
		if legalFineValues[tt.dt] && tt.work == tt.rest {
			t.Errorf("合法值 %s(%d) IsWork/IsRest 必须恰一为真", tt.name, tt.dt)
		}
	}
}

// TestComboPredicates 组合判断方法：各方法仅对其组合值返回 true。
func TestComboPredicates(t *testing.T) {
	for _, tt := range validFineValues {
		cases := []struct {
			name string
			got  bool
		}{
			{"IsFestivalRest", tt.dt.IsFestivalRest()},
			{"IsAdjustedRestDay", tt.dt.IsAdjustedRestDay()},
			{"IsAdjustedWorkDay", tt.dt.IsAdjustedWorkDay()},
		}
		wants := map[string]string{
			"IsFestivalRest":    "FestivalRest",
			"IsAdjustedRestDay": "AdjustedRestDay",
			"IsAdjustedWorkDay": "AdjustedWorkDay",
		}
		for _, c := range cases {
			if want := tt.name == wants[c.name]; c.got != want {
				t.Errorf("%s(%d).%s() = %v, want %v", tt.name, tt.dt, c.name, c.got, want)
			}
		}
	}
}

// TestIsValid 合法值校验：五值 true，其余值 false。
func TestIsValid(t *testing.T) {
	for _, tt := range validFineValues {
		if !tt.dt.IsValid() {
			t.Errorf("%s(%d).IsValid() = false, want true", tt.name, tt.dt)
		}
	}
	for v := 0; v <= 255; v++ {
		dt := goliday.DayType(v)
		if legalFineValues[dt] {
			continue
		}
		if dt.IsValid() {
			t.Errorf("DayType(%d).IsValid() = true, want false", v)
		}
	}
}

// TestString String() 的字符串断言（粗粒度值 1/2 天然输出 work/rest）。
func TestString(t *testing.T) {
	for _, tt := range validFineValues {
		if got := tt.dt.String(); got != tt.str {
			t.Errorf("%s.String() = %q, want %q", tt.name, got, tt.str)
		}
	}
	if got := goliday.DayTypeWork.String(); got != "work" {
		t.Errorf("DayTypeWork.String() = %q, want %q", got, "work")
	}
	if got := goliday.DayTypeRest.String(); got != "rest" {
		t.Errorf("DayTypeRest.String() = %q, want %q", got, "rest")
	}
}

// TestLegalValueBitAndDisjoint 对全部合法值断言 t & Work 与 t & Rest
// 恰一非零（单次位与判类无歧义）。
func TestLegalValueBitAndDisjoint(t *testing.T) {
	for _, tt := range validFineValues {
		hasWork := tt.dt&goliday.DayTypeWork != 0
		hasRest := tt.dt&goliday.DayTypeRest != 0
		if hasWork == hasRest {
			t.Errorf("%s(%d) 位与判类必须恰一非零: work=%v rest=%v",
				tt.name, tt.dt, hasWork, hasRest)
		}
		if hasWork != tt.dt.IsWork() || hasRest != tt.dt.IsRest() {
			t.Errorf("%s(%d) 位与判类与 IsWork/IsRest 不一致", tt.name, tt.dt)
		}
	}
}

// TestDayTypeExhaustiveInvariants 穷举 uint8 全部 256 个取值，验证
// String 输出的每个分段均为合法位名或 unknown、不 panic；
// 对 5 个合法值另断言 Coarse 幂等且 ∈ {Work, Rest}、
// IsWork/IsRest 恰一为真、位与判类恰一非零。
func TestDayTypeExhaustiveInvariants(t *testing.T) {
	legalNames := map[string]bool{
		"work": true, "rest": true, "festival": true,
		"adjusted": true, "compensate": true, "unknown": true,
	}
	for v := 0; v <= 255; v++ {
		dt := goliday.DayType(v)
		t.Run(dt.String(), func(t *testing.T) {
			for part := range strings.SplitSeq(dt.String(), "|") {
				if !legalNames[part] {
					t.Fatalf("DayType(%d).String() = %q 含非法分段 %q", v, dt.String(), part)
				}
			}
			if !legalFineValues[dt] {
				return
			}
			coarse := dt.Coarse()
			if coarse != goliday.DayTypeWork && coarse != goliday.DayTypeRest {
				t.Fatalf("合法值 %d 的 Coarse() = %d，必须 ∈ {Work, Rest}", v, coarse)
			}
			if coarse.Coarse() != coarse {
				t.Fatalf("Coarse 不幂等: %d → %d → %d", v, coarse, coarse.Coarse())
			}
			if dt.IsWork() == dt.IsRest() {
				t.Fatalf("合法值 %d IsWork/IsRest 必须恰一为真", v)
			}
			hasWork := dt&goliday.DayTypeWork != 0
			hasRest := dt&goliday.DayTypeRest != 0
			if hasWork == hasRest {
				t.Fatalf("合法值 %d 位与判类必须恰一非零", v)
			}
		})
	}
}
