package goliday

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mustLoadDir 加载目录，失败时终止测试。
func mustLoadDir(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := LoadDir(dir)
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

func date(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := parseDate(s)
	if err != nil {
		t.Fatalf("解析日期 %s 失败: %v", s, err)
	}
	return d
}

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
	var spring *Festival
	for i, f := range c26.Festivals {
		if f.Name == "春节" {
			spring = &c26.Festivals[i]
			break
		}
	}
	if spring == nil {
		t.Fatal("2026 未找到 festival 春节")
	}
	if got, want := spring.Date.Format(dateLayout), "2026-02-17"; got != want {
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
	cfg, err := LoadYear(filepath.Join("testdata", "2026.toml"))
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
			cfg, err := LoadYear(path)
			if err == nil {
				t.Fatalf("LoadYear(%q) 未报错，返回 %+v", path, cfg)
			}
			msg := err.Error()
			if !strings.Contains(msg, filepath.ToSlash(path)) {
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

func TestLeapYearValid(t *testing.T) {
	dir := t.TempDir()
	path := writeYearFile(t, dir, "2028.toml", leapYearTOML)

	cfg, err := LoadYear(path)
	if err != nil {
		t.Fatalf("LoadYear(%q) 报错: %v", path, err)
	}
	if !contains(cfg.Adjust.Off, date(t, "2028-02-29")) {
		t.Error("2028 off 应含闰日 2028-02-29")
	}
}

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

func TestValidateRules(t *testing.T) {
	parse := func(s string) time.Time {
		d, err := parseDate(s)
		if err != nil {
			t.Fatalf("解析 %s 失败: %v", s, err)
		}
		return d
	}

	cases := []struct {
		name string
		cfg  YearConfig
		kw   string
	}{
		{
			name: "off 与 work 重复",
			// 2026-01-05 为周一，同日出现在 off 与 work 中。
			cfg: YearConfig{
				Year:   2026,
				Adjust: Adjust{Off: []time.Time{parse("2026-01-05")}, Work: []time.Time{parse("2026-01-05")}},
			},
			kw: "off",
		},
		{
			name: "work 含节日当天",
			cfg: YearConfig{
				Year:      2026,
				Festivals: []Festival{{Name: "元旦", Date: parse("2026-01-03")}},
				// 2026-01-03 为周六。
				Adjust: Adjust{Work: []time.Time{parse("2026-01-03")}},
			},
			kw: "补班",
		},
		{
			name: "work 日期重复",
			cfg: YearConfig{
				Year:   2026,
				Adjust: Adjust{Work: []time.Time{parse("2026-01-03"), parse("2026-01-03")}},
			},
			kw: "重复",
		},
		{
			name: "festival 日期重复",
			cfg: YearConfig{
				Year:      2026,
				Festivals: []Festival{{Name: "元旦", Date: parse("2026-01-01")}, {Name: "另一节日", Date: parse("2026-01-01")}},
			},
			kw: "重复",
		},
		{
			name: "festival 年份不符",
			cfg: YearConfig{
				Year:      2026,
				Festivals: []Festival{{Name: "元旦", Date: parse("2025-01-01")}},
			},
			kw: "年份",
		},
		{
			name: "off 年份不符",
			cfg: YearConfig{
				Year:   2026,
				Adjust: Adjust{Off: []time.Time{parse("2025-01-01")}},
			},
			kw: "年份",
		},
		{
			name: "work 年份不符",
			cfg: YearConfig{
				Year:   2026,
				Adjust: Adjust{Work: []time.Time{parse("2025-01-04")}},
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
		if _, err := parseDate(s); err == nil {
			t.Errorf("parseDate(%q) 未报错，期望报错", s)
		}
	}
	if _, err := parseDate("2026-02-28"); err != nil {
		t.Errorf("parseDate(\"2026-02-28\") 报错: %v", err)
	}
}

func TestLoadYearBadFilename(t *testing.T) {
	path := writeYearFile(t, t.TempDir(), "holiday.toml", minimal2026TOML)
	_, err := LoadYear(path)
	if err == nil {
		t.Fatal("非年份命名文件 LoadYear 未报错")
	}
	if !strings.Contains(err.Error(), "文件名") {
		t.Errorf("错误信息 %q 未包含关键词 文件名", err)
	}
}

func TestLoadYearMissingFile(t *testing.T) {
	if _, err := LoadYear(filepath.Join(t.TempDir(), "2099.toml")); err == nil {
		t.Error("不存在的文件 LoadYear 未报错")
	}
}
