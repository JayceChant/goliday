package goliday

import "testing"

// validFineCombos 细粒度合法组合全集（6 种）及其期望数值与字符串。
var validFineCombos = []struct {
	name string
	dt   DayType
	val  DayType
	str  string
}{
	{"Ordinary", DayTypeOrdinary, 1, "ordinary"},
	{"Weekend", DayTypeWeekend, 4, "weekend"},
	{"Compensate|Weekend", DayTypeCompensate | DayTypeWeekend, 6, "compensate|weekend"},
	{"Festival|Weekend", DayTypeFestival | DayTypeWeekend, 12, "weekend|festival"},
	{"Adjusted", DayTypeAdjusted, 16, "adjusted"},
	{"Festival|Adjusted", DayTypeFestival | DayTypeAdjusted, 24, "festival|adjusted"},
}

// TestFineGrainedValues 细粒度合法组合全集的数值断言。
func TestFineGrainedValues(t *testing.T) {
	for _, tt := range validFineCombos {
		if tt.dt != tt.val {
			t.Errorf("%s = %d, want %d", tt.name, tt.dt, tt.val)
		}
	}
	if DayTypeWorkday != 3 {
		t.Errorf("DayTypeWorkday = %d, want 3", DayTypeWorkday)
	}
	if DayTypeHoliday != 28 {
		t.Errorf("DayTypeHoliday = %d, want 28", DayTypeHoliday)
	}
}

// TestCoarse Coarse() 的粗粒度映射断言。
func TestCoarse(t *testing.T) {
	tests := []struct {
		name string
		dt   DayType
		want DayType
	}{
		{"Ordinary", DayTypeOrdinary, DayTypeWorkday},
		{"Compensate|Weekend", DayTypeCompensate | DayTypeWeekend, DayTypeWorkday},
		{"Weekend", DayTypeWeekend, DayTypeHoliday},
		{"Festival|Weekend", DayTypeFestival | DayTypeWeekend, DayTypeHoliday},
		{"Adjusted", DayTypeAdjusted, DayTypeHoliday},
		{"Festival|Adjusted", DayTypeFestival | DayTypeAdjusted, DayTypeHoliday},
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
		dt      DayType
		workday bool
		holiday bool
	}{
		{"Workday(粗粒度)", DayTypeWorkday, true, false},
		{"Holiday(粗粒度)", DayTypeHoliday, false, true},
		{"Ordinary", DayTypeOrdinary, true, false},
		{"Weekend", DayTypeWeekend, false, true},
		{"Compensate|Weekend", DayTypeCompensate | DayTypeWeekend, true, false},
		{"Festival|Weekend", DayTypeFestival | DayTypeWeekend, false, true},
		{"Adjusted", DayTypeAdjusted, false, true},
		{"Festival|Adjusted", DayTypeFestival | DayTypeAdjusted, false, true},
	}
	for _, tt := range all {
		if wantWorkday := tt.dt.Coarse() == DayTypeWorkday; tt.dt.IsWorkday() != wantWorkday {
			t.Errorf("%s.IsWorkday() = %v, want %v", tt.name, tt.dt.IsWorkday(), wantWorkday)
		}
		if tt.dt.IsWorkday() != tt.workday {
			t.Errorf("%s.IsWorkday() = %v, want %v", tt.name, tt.dt.IsWorkday(), tt.workday)
		}
		if wantHoliday := tt.dt.Coarse() == DayTypeHoliday; tt.dt.IsHoliday() != wantHoliday {
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
	if got := DayTypeWorkday.String(); got != "workday" {
		t.Errorf("DayTypeWorkday.String() = %q, want %q", got, "workday")
	}
	if got := DayTypeHoliday.String(); got != "holiday" {
		t.Errorf("DayTypeHoliday.String() = %q, want %q", got, "holiday")
	}
}
