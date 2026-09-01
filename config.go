package goliday

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
)

// Festival 描述一个法定节日：名称与节日当天日期。
type Festival struct {
	Name string
	Date time.Time
}

// Adjust 描述某年的调休调整（稀疏表）：
//
//	Off  放假日：原本需要上班（周一~周五）但被调整为休息的日期，含节日当天与调休日；
//	Work 补班日：原本休息（周六/周日）但被调整为上班的日期。
type Adjust struct {
	Off  []time.Time
	Work []time.Time
}

// YearConfig 描述某一年的节假日稀疏配置。
type YearConfig struct {
	Year      int
	Name      string
	Festivals []Festival
	Adjust    Adjust
}

// dateLayout 配置文件中的日期格式，严格要求 YYYY-MM-DD。
const dateLayout = "2006-01-02"

// tomlFestival TOML 中 [[festival]] 的中间结构。
type tomlFestival struct {
	Name string `toml:"name"`
	Date string `toml:"date"`
}

// tomlAdjust TOML 中 [adjust] 的中间结构。
type tomlAdjust struct {
	Off  []string `toml:"off"`
	Work []string `toml:"work"`
}

// tomlConfig TOML 文件的中间结构。
type tomlConfig struct {
	Year      int            `toml:"year"`
	Name      string         `toml:"name"`
	Festivals []tomlFestival `toml:"festival"`
	Adjust    tomlAdjust     `toml:"adjust"`
}

// parseDate 严格解析 YYYY-MM-DD 日期字符串。
func parseDate(s string) (time.Time, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("非法日期 %q：须为 YYYY-MM-DD 格式的有效日期", s)
	}
	// time.Parse 对 "2026-02-30" 这类不存在的日期会进位而非报错，须额外校验。
	if t.Format(dateLayout) != s {
		return time.Time{}, fmt.Errorf("非法日期 %q：该日期不存在", s)
	}
	return t, nil
}

// toYearConfig 将中间结构转换为导出的 YearConfig。
func (tc *tomlConfig) toYearConfig() (*YearConfig, error) {
	cfg := &YearConfig{
		Year: tc.Year,
		Name: tc.Name,
	}
	for i, f := range tc.Festivals {
		d, err := parseDate(f.Date)
		if err != nil {
			return nil, fmt.Errorf("festival[%d]（%s）%w", i, f.Name, err)
		}
		cfg.Festivals = append(cfg.Festivals, Festival{Name: f.Name, Date: d})
	}
	for i, s := range tc.Adjust.Off {
		d, err := parseDate(s)
		if err != nil {
			return nil, fmt.Errorf("adjust.off[%d] %w", i, err)
		}
		cfg.Adjust.Off = append(cfg.Adjust.Off, d)
	}
	for i, s := range tc.Adjust.Work {
		d, err := parseDate(s)
		if err != nil {
			return nil, fmt.Errorf("adjust.work[%d] %w", i, err)
		}
		cfg.Adjust.Work = append(cfg.Adjust.Work, d)
	}
	return cfg, nil
}

// weekdayCN 返回中文星期名，用于错误信息。
func weekdayCN(t time.Time) string {
	names := [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	return names[int(t.Weekday())]
}

// Validate 校验 YearConfig 是否符合稀疏表约束，错误信息指明文件内具体原因：
//
//   - 所有日期年份须等于 c.Year；
//   - off 中日期须为周一~周五，work 中日期须为周六/周日；
//   - off、work 各自无重复，且两者互斥；
//   - work 不得包含任何 festival.date（节日当天不得补班）；
//   - festival.date 为周一~周五时须在 off 中（节日当天为工作日必放假，
//     否则判型得 Ordinary|Festival，不在合法细粒度组合全集内）；
//   - festival.date 之间无重复。
func (c *YearConfig) Validate() error {
	// festival：年份匹配、无重复。
	seenFestival := make(map[time.Time]string, len(c.Festivals))
	for _, f := range c.Festivals {
		if y := f.Date.Year(); y != c.Year {
			return fmt.Errorf("festival %q 日期 %s 年份为 %d，与 year = %d 不符",
				f.Name, f.Date.Format(dateLayout), y, c.Year)
		}
		if prev, dup := seenFestival[f.Date]; dup {
			return fmt.Errorf("festival 日期重复 %s（%s 与 %s）",
				f.Date.Format(dateLayout), prev, f.Name)
		}
		seenFestival[f.Date] = f.Name
	}

	// off：年份匹配、周一~周五、无重复。
	seenOff := make(map[time.Time]bool, len(c.Adjust.Off))
	for _, d := range c.Adjust.Off {
		if y := d.Year(); y != c.Year {
			return fmt.Errorf("off 日期 %s 年份为 %d，与 year = %d 不符",
				d.Format(dateLayout), y, c.Year)
		}
		wd := d.Weekday()
		if wd == time.Saturday || wd == time.Sunday {
			return fmt.Errorf("off 含周末日期 %s（%s）：违反稀疏表原则",
				d.Format(dateLayout), weekdayCN(d))
		}
		if seenOff[d] {
			return fmt.Errorf("off 日期重复 %s", d.Format(dateLayout))
		}
		seenOff[d] = true
	}

	// work：年份匹配、周六/周日、无重复、与 off 互斥、不含节日当天。
	seenWork := make(map[time.Time]bool, len(c.Adjust.Work))
	for _, d := range c.Adjust.Work {
		if y := d.Year(); y != c.Year {
			return fmt.Errorf("work 日期 %s 年份为 %d，与 year = %d 不符",
				d.Format(dateLayout), y, c.Year)
		}
		if seenOff[d] {
			return fmt.Errorf("日期 %s 同时出现在 off 与 work 中", d.Format(dateLayout))
		}
		if seenWork[d] {
			return fmt.Errorf("work 日期重复 %s", d.Format(dateLayout))
		}
		seenWork[d] = true
		wd := d.Weekday()
		if wd != time.Saturday && wd != time.Sunday {
			return fmt.Errorf("work 含工作日日期 %s（%s）：违反稀疏表原则",
				d.Format(dateLayout), weekdayCN(d))
		}
		if _, isFestival := seenFestival[d]; isFestival {
			return fmt.Errorf("work 日期 %s 为节日当天，节日当天不得补班", d.Format(dateLayout))
		}
	}

	// festival 当天为工作日（周一~周五）时必须在 off 中：节日当天必放假，
	// 否则判型为 Ordinary|Festival（9），不在合法细粒度组合全集内。
	for _, f := range c.Festivals {
		if _, ok := seenOff[f.Date]; ok {
			continue
		}
		if wd := f.Date.Weekday(); wd != time.Saturday && wd != time.Sunday {
			return fmt.Errorf("festival %q 日期 %s（%s）为工作日但不在 off 中：节日当天为工作日须调整为休息",
				f.Name, f.Date.Format(dateLayout), weekdayCN(f.Date))
		}
	}

	return nil
}

// yearFilePattern 匹配纯四位数字年份命名的 toml 文件，如 2026.toml。
var yearFilePattern = regexp.MustCompile(`^\d{4}\.toml$`)

// LoadYear 读取并解析单个年份配置文件，执行校验后返回 YearConfig。
//
// 文件名（去扩展名）须为纯四位数字年份，且与文件内 year 字段一致。
// 返回的错误信息以文件路径开头，便于定位。
func LoadYear(path string) (*YearConfig, error) {
	var tc tomlConfig
	if _, err := toml.DecodeFile(path, &tc); err != nil {
		return nil, fmt.Errorf("%s: 解析 TOML 失败: %w", path, err)
	}

	cfg, err := tc.toYearConfig()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	base := filepath.Base(path)
	stem := base[:len(base)-len(filepath.Ext(base))]
	if !yearFilePattern.MatchString(base) {
		return nil, fmt.Errorf("%s: 文件名须为纯四位数字年份（如 2026.toml）", path)
	}
	if fileYear, err := strconv.Atoi(stem); err != nil || fileYear != cfg.Year {
		return nil, fmt.Errorf("%s: 文件名年份 %s 与 year = %d 不一致", path, stem, cfg.Year)
	}

	return cfg, nil
}
