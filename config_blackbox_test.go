// 黑盒测试（package goliday_test）：仅经导出 API（LoadYear/LoadDir/
// YearConfig.Validate）验证配置加载与校验的对外契约；本文件同时提供
// 黑盒测试公用 helper 与内联 TOML 常量。
package goliday_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goliday"
)

// layout 黑盒侧自带的 YYYY-MM-DD 格式（根包 dateLayout 为未导出标识符）。
const layout = "2006-01-02"

// mustLoadDir 加载目录，失败时终止测试。
func mustLoadDir(t *testing.T, dir string) *goliday.Store {
	t.Helper()
	s, err := goliday.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir(%q) 报错: %v", dir, err)
	}
	return s
}

func contains(list []time.Time, want time.Time) bool {
	for _, d := range list {
		if d.Equal(want) {
			return true
		}
	}
	return false
}

// date 黑盒侧日期构造：仅用标准库解析 YYYY-MM-DD。
func date(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse(layout, s)
	if err != nil {
		t.Fatalf("解析日期 %s 失败: %v", s, err)
	}
	return d
}

// writeYearFile 在临时目录写一个年份配置文件。
func writeYearFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入 %s 失败: %v", path, err)
	}
	return path
}

const leapYearTOML = `year = 2028

[[festival]]
name = "元旦"
date = "2028-01-01"

[adjust]
off = [ "2028-01-03", "2028-02-29" ]
work = [ "2028-01-29" ]
`

const minimal2025TOML = `year = 2025

[[festival]]
name = "元旦"
date = "2025-01-01"

[adjust]
off = [ "2025-01-01" ]
work = [ "2025-01-26" ]
`

const minimal2026TOML = `year = 2026

[[festival]]
name = "元旦"
date = "2026-01-01"

[adjust]
off = [ "2026-01-01", "2026-01-02" ]
work = [ ]
`

const minimal2027TOML = `year = 2027

[[festival]]
name = "元旦"
date = "2027-01-01"

[adjust]
off = [ "2027-01-01" ]
work = [ ]
`

func TestLoadDirTestData(t *testing.T) {
	s := mustLoadDir(t, "testdata")

	if got, want := s.Years(), []int{2025, 2026}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Years() = %v，期望 %v", got, want)
	}
	if !s.Has(2025) || !s.Has(2026) {
		t.Error("Has(2025)/Has(2026) 应均为 true")
	}
	if s.Has(2027) || s.Get(2027) != nil {
		t.Error("Has(2027)/Get(2027) 应为 false/nil")
	}

	// 2025 年内容断言。
	c25 := s.Get(2025)
	if c25 == nil {
		t.Fatal("Get(2025) = nil")
	}
	if c25.Year != 2025 {
		t.Errorf("c25.Year = %d，期望 2025", c25.Year)
	}
	if n := len(c25.Festivals); n != 7 {
		t.Errorf("2025 Festivals 数量 = %d，期望 7", n)
	}
	if n := len(c25.Adjust.Off); n != 18 {
		t.Errorf("2025 Adjust.Off 数量 = %d，期望 18", n)
	}
	if n := len(c25.Adjust.Work); n != 5 {
		t.Errorf("2025 Adjust.Work 数量 = %d，期望 5", n)
	}
	if !contains(c25.Adjust.Off, date(t, "2025-01-29")) {
		t.Error("2025 off 应含 2025-01-29（春节当天）")
	}
	if !contains(c25.Adjust.Work, date(t, "2025-01-26")) {
		t.Error("2025 work 应含 2025-01-26")
	}

	// 2026 年内容断言。
	c26 := s.Get(2026)
	if c26 == nil {
		t.Fatal("Get(2026) = nil")
	}
	if n := len(c26.Festivals); n != 7 {
		t.Errorf("2026 Festivals 数量 = %d，期望 7", n)
	}
	var spring *goliday.Festival
	for i, f := range c26.Festivals {
		if f.Name == "春节" {
			spring = &c26.Festivals[i]
			break
		}
	}
	if spring == nil {
		t.Fatal("2026 未找到 festival 春节")
	}
	if got, want := spring.Date.Format(layout), "2026-02-17"; got != want {
		t.Errorf("2026 春节日期 = %s，期望 %s", got, want)
	}
	if n := len(c26.Adjust.Off); n != 19 {
		t.Errorf("2026 Adjust.Off 数量 = %d，期望 19", n)
	}
	if n := len(c26.Adjust.Work); n != 5 {
		t.Errorf("2026 Adjust.Work 数量 = %d，期望 5", n)
	}
	if !contains(c26.Adjust.Off, date(t, "2026-02-20")) {
		t.Error("2026 off 应含 2026-02-20")
	}
	if !contains(c26.Adjust.Work, date(t, "2026-02-28")) {
		t.Error("2026 work 应含 2026-02-28")
	}
}

func TestLoadYear2026(t *testing.T) {
	cfg, err := goliday.LoadYear(filepath.Join("testdata", "2026.toml"))
	if err != nil {
		t.Fatalf("LoadYear 报错: %v", err)
	}
	if cfg.Year != 2026 {
		t.Errorf("cfg.Year = %d，期望 2026", cfg.Year)
	}
	if len(cfg.Festivals) != 7 || len(cfg.Adjust.Off) != 19 || len(cfg.Adjust.Work) != 5 {
		t.Errorf("数量断言失败：festival=%d off=%d work=%d，期望 7/19/5",
			len(cfg.Festivals), len(cfg.Adjust.Off), len(cfg.Adjust.Work))
	}
	if !contains(cfg.Adjust.Off, date(t, "2026-02-17")) {
		t.Error("2026 off 应含春节当天 2026-02-17")
	}
}

// invalidFiles 每个 invalid 样例的文件名与错误信息应含的关键词。
// 注：year_mismatch 经 LoadYear 直读时触发"文件名非纯数字年份"错误（含"年份"），
// 经 LoadDir（重命名为 2026.toml）时触发"文件名年份与 year 不一致"错误。
var invalidFiles = map[string][]string{
	"year_mismatch.toml": {"年份"},
	"invalid_date.toml":  {"非法日期"},
	"off_weekend.toml":   {"周末"},
	"work_weekday.toml":  {"工作日"},
	"dup.toml":           {"重复"},
}

func TestLoadYearInvalid(t *testing.T) {
	for name, keywords := range invalidFiles {
		name, keywords := name, keywords
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", "invalid", name)
			cfg, err := goliday.LoadYear(path)
			if err == nil {
				t.Fatalf("LoadYear(%q) 未报错，返回 %+v", path, cfg)
			}
			msg := err.Error()
			// 错误信息内嵌的即传入的原始路径，直接断言（兼容平台路径分隔符差异）。
			if !strings.Contains(msg, path) {
				t.Errorf("错误信息 %q 未包含文件名 %q", msg, path)
			}
			for _, kw := range keywords {
				if !strings.Contains(msg, kw) {
					t.Errorf("错误信息 %q 未包含关键词 %q", msg, kw)
				}
			}
		})
	}
}

func TestLeapYearValid(t *testing.T) {
	dir := t.TempDir()
	path := writeYearFile(t, dir, "2028.toml", leapYearTOML)

	cfg, err := goliday.LoadYear(path)
	if err != nil {
		t.Fatalf("LoadYear(%q) 报错: %v", path, err)
	}
	if !contains(cfg.Adjust.Off, date(t, "2028-02-29")) {
		t.Error("2028 off 应含闰日 2028-02-29")
	}
}

func TestValidateRules(t *testing.T) {
	cases := []struct {
		name string
		cfg  goliday.YearConfig
		kw   string
	}{
		{
			name: "off 与 work 重复",
			// 2026-01-05 为周一，同日出现在 off 与 work 中。
			cfg: goliday.YearConfig{
				Year:   2026,
				Adjust: goliday.Adjust{Off: []time.Time{date(t, "2026-01-05")}, Work: []time.Time{date(t, "2026-01-05")}},
			},
			kw: "off",
		},
		{
			name: "work 含节日当天",
			cfg: goliday.YearConfig{
				Year:      2026,
				Festivals: []goliday.Festival{{Name: "元旦", Date: date(t, "2026-01-03")}},
				// 2026-01-03 为周六。
				Adjust: goliday.Adjust{Work: []time.Time{date(t, "2026-01-03")}},
			},
			kw: "补班",
		},
		{
			name: "work 日期重复",
			cfg: goliday.YearConfig{
				Year:   2026,
				Adjust: goliday.Adjust{Work: []time.Time{date(t, "2026-01-03"), date(t, "2026-01-03")}},
			},
			kw: "重复",
		},
		{
			name: "festival 日期重复",
			cfg: goliday.YearConfig{
				Year:      2026,
				Festivals: []goliday.Festival{{Name: "元旦", Date: date(t, "2026-01-01")}, {Name: "另一节日", Date: date(t, "2026-01-01")}},
			},
			kw: "重复",
		},
		{
			name: "festival 年份不符",
			cfg: goliday.YearConfig{
				Year:      2026,
				Festivals: []goliday.Festival{{Name: "元旦", Date: date(t, "2025-01-01")}},
			},
			kw: "年份",
		},
		{
			name: "off 年份不符",
			cfg: goliday.YearConfig{
				Year:   2026,
				Adjust: goliday.Adjust{Off: []time.Time{date(t, "2025-01-01")}},
			},
			kw: "年份",
		},
		{
			name: "work 年份不符",
			cfg: goliday.YearConfig{
				Year:   2026,
				Adjust: goliday.Adjust{Work: []time.Time{date(t, "2025-01-04")}},
			},
			kw: "年份",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if err == nil {
				t.Fatal("Validate() 未报错")
			}
			if !strings.Contains(err.Error(), tc.kw) {
				t.Errorf("错误信息 %q 未包含关键词 %q", err, tc.kw)
			}
		})
	}
}

func TestLoadYearBadFilename(t *testing.T) {
	path := writeYearFile(t, t.TempDir(), "holiday.toml", minimal2026TOML)
	_, err := goliday.LoadYear(path)
	if err == nil {
		t.Fatal("非年份命名文件 LoadYear 未报错")
	}
	if !strings.Contains(err.Error(), "文件名") {
		t.Errorf("错误信息 %q 未包含关键词 文件名", err)
	}
}

func TestLoadYearMissingFile(t *testing.T) {
	if _, err := goliday.LoadYear(filepath.Join(t.TempDir(), "2099.toml")); err == nil {
		t.Error("不存在的文件 LoadYear 未报错")
	}
}

// FuzzLoadYearTOML 不变量：任意年份 + TOML 文本，LoadYear 成功时
// 配置必须满足稀疏表全部约束，且经 LoadDir 构造的 Calendar 对
// off/work 日期判型正确；失败时返回明确错误（由 fuzz 引擎检测 panic）。
func FuzzLoadYearTOML(f *testing.F) {
	f.Add(2026, minimal2026TOML)
	f.Add(2028, leapYearTOML)
	f.Add(2026, `year = 2026
[[festival]]
name = "元旦"
date = "2026-01-01"

[adjust]
off = [ "2026-01-03" ]
work = [ "2026-01-31" ]
`) // off 含周末：应被拒绝
	f.Add(2025, minimal2026TOML) // 文件名年份与内容 year 不一致：应被拒绝
	f.Add(2026, "not a toml {{{")
	f.Fuzz(func(t *testing.T, year int, body string) {
		// 归一化到合法文件名年份域（NNNN.toml）。
		if year < 1000 || year > 9999 {
			t.Skip()
		}
		dir := t.TempDir()
		path := writeYearFile(t, dir, fmt.Sprintf("%04d.toml", year), body)

		cfg, err := goliday.LoadYear(path)
		if err != nil {
			return // 任意 TOML 允许失败
		}

		// 成功则必须满足全部不变量。
		if err := cfg.Validate(); err != nil {
			t.Fatalf("LoadYear 成功但 Validate 失败: %v", err)
		}
		if cfg.Year != year {
			t.Fatalf("cfg.Year = %d，与文件名 %d 不一致", cfg.Year, year)
		}
		seen := map[time.Time]bool{}
		for _, d := range cfg.Adjust.Off {
			if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
				t.Fatalf("off 含周末 %s", d.Format(layout))
			}
			if d.Year() != cfg.Year {
				t.Fatalf("off 日期 %s 不在 %d 年内", d.Format(layout), cfg.Year)
			}
			if seen[d] {
				t.Fatalf("off 日期重复 %s", d.Format(layout))
			}
			seen[d] = true
		}
		for _, d := range cfg.Adjust.Work {
			if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
				t.Fatalf("work 含工作日 %s", d.Format(layout))
			}
			if d.Year() != cfg.Year {
				t.Fatalf("work 日期 %s 不在 %d 年内", d.Format(layout), cfg.Year)
			}
			if seen[d] {
				t.Fatalf("work 日期 %s 重复或与 off 相交", d.Format(layout))
			}
			seen[d] = true
		}

		// 加载为 Store/Calendar 后，off/work 判型与配置语义一致。
		s, err := goliday.LoadDir(dir)
		if err != nil {
			t.Fatalf("LoadYear 成功但 LoadDir 失败: %v", err)
		}
		c := goliday.NewCalendar(s)
		for _, d := range cfg.Adjust.Off {
			if got := c.Query(d); got&goliday.DayTypeAdjusted == 0 {
				t.Fatalf("off 日期 %s 判型 = %d（%s），应含 Adjusted 位",
					d.Format(layout), got, got)
			}
		}
		for _, d := range cfg.Adjust.Work {
			if got, want := c.Query(d), goliday.DayTypeCompensate|goliday.DayTypeWeekend; got != want {
				t.Fatalf("work 日期 %s 判型 = %d（%s），期望 %d（%s）",
					d.Format(layout), got, got, want, want)
			}
		}
	})
}
