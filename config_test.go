// 白盒测试（package goliday）：覆盖严格 YYYY-MM-DD 解析（导出的
// ParseDate）的边界与 fuzz 不变量。
package goliday

import (
	"testing"
	"time"
)

func TestParseDateStrict(t *testing.T) {
	bad := []string{
		"2026-2-30",  // 非补零
		"2026-02-30", // 不存在的日期
		"2026-13-01", // 不存在的月份
		"26-02-01",   // 年份非四位
		"2026/02/01", // 错误分隔符
		"",           // 空串
	}
	for _, s := range bad {
		if _, err := ParseDate(s); err == nil {
			t.Errorf("ParseDate(%q) 未报错，期望报错", s)
		}
	}
	if _, err := ParseDate("2026-02-28"); err != nil {
		t.Errorf("ParseDate(\"2026-02-28\") 报错: %v", err)
	}
}

// FuzzParseDate 不变量：ParseDate 成功 ⇔ time.Parse 接受且回格式化一致；
// 成功值再解析幂等；失败必须返回非 nil 错误。
func FuzzParseDate(f *testing.F) {
	for _, s := range []string{
		"2026-02-28", "2026-02-30", "2028-02-29", "2027-02-29",
		"2026-2-28", "26-02-28", "2026/02/28", "", "2026-12-31",
		"0001-01-01", "9999-12-31", "2026-00-10", "2026-13-01", "2026-01-32",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got, err := ParseDate(s)

		want, wantErr := time.Parse(dateLayout, s)
		if wantErr != nil || want.Format(dateLayout) != s {
			// 期望失败：必须报错。
			if err == nil {
				t.Fatalf("ParseDate(%q) 成功，期望失败", s)
			}
			return
		}
		// 期望成功：必须成功且结果一致。
		if err != nil {
			t.Fatalf("ParseDate(%q) 报错: %v，期望成功", s, err)
		}
		if !got.Equal(want) {
			t.Fatalf("ParseDate(%q) = %v，期望 %v", s, got, want)
		}
		if again, err2 := ParseDate(got.Format(dateLayout)); err2 != nil || !again.Equal(got) {
			t.Fatalf("ParseDate(%q) 再解析不幂等: %v, %v", s, again, err2)
		}
	})
}
