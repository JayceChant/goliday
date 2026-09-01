// 白盒测试（package main）：fuzz 任意年份与公告文本，验证 gen 的
// 解析与草稿组装恒满足稀疏表不变量。
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JayceChant/goliday"
)

// FuzzGenDraft 不变量：解析条目区间有效且在年内；草稿 off 全为周一~五、
// work 全为周六/日、互斥无重复、全在年内；festival 日期非 TODO 则为合法
// YYYY-MM-DD 且在年内；selfCheck 失败仅允许 TODO 占位、festival 日期重复
// 或"节日当天不得补班"；自检通过时 render 产物可被根包 LoadYear 加载。
func FuzzGenDraft(f *testing.F) {
	f.Add(2026, ann2026)
	f.Add(2026, "一、端午节：6月19日至21日放假，共3天。")
	f.Add(2024, "二、春节：2月10日至17日放假调休，共8天。2月4日（星期日）、2月18日（星期日）上班。")
	f.Add(2026, "本通知自发布之日起执行。")
	f.Add(2027, "")
	f.Fuzz(func(t *testing.T, year int, text string) {
		// 归一化到可写入 NNNN.toml 文件名的年份域。
		if year < 1000 || year > 9999 {
			t.Skip()
		}

		res := parseAnnouncement(year, text)
		d := buildDraft(year, res, lunarMarks(text))

		for _, e := range res.entries {
			if e.end.Before(e.start) {
				t.Fatalf("条目区间倒挂: %s ~ %s", e.start, e.end)
			}
			if e.start.Year() != year || e.end.Year() != year {
				t.Fatalf("条目区间跨年: %s ~ %s（year=%d）", e.start, e.end, year)
			}
		}

		seenOff := map[time.Time]bool{}
		for _, d0 := range d.off {
			if wd := d0.Weekday(); wd == time.Saturday || wd == time.Sunday {
				t.Fatalf("off 含周末 %s", d0.Format(dateLayoutTool))
			}
			if d0.Year() != year {
				t.Fatalf("off 日期 %s 不在 %d 年内", d0, year)
			}
			if seenOff[d0] {
				t.Fatalf("off 日期重复 %s", d0)
			}
			seenOff[d0] = true
		}
		seenWork := map[time.Time]bool{}
		for _, w := range d.work {
			if wd := w.Weekday(); wd != time.Saturday && wd != time.Sunday {
				t.Fatalf("work 含工作日 %s", w.Format(dateLayoutTool))
			}
			if w.Year() != year {
				t.Fatalf("work 日期 %s 不在 %d 年内", w, year)
			}
			if seenWork[w] {
				t.Fatalf("work 日期重复 %s", w)
			}
			if seenOff[w] {
				t.Fatalf("日期 %s 同时出现在 off 与 work 中", w)
			}
			seenWork[w] = true
		}

		hasTODO := false
		for _, fs := range d.festivals {
			if fs.Date == dateTODO {
				hasTODO = true
				continue
			}
			pt, err := time.Parse(dateLayoutTool, fs.Date)
			if err != nil {
				t.Fatalf("festival %s 日期 %q 非法: %v", fs.Name, fs.Date, err)
			}
			if pt.Year() != year {
				t.Fatalf("festival %s 日期 %s 不在 %d 年内", fs.Name, fs.Date, year)
			}
		}

		err := selfCheck(d)
		switch {
		case err == nil, errors.Is(err, errFestivalTODO):
			// 通过或仅因 TODO 占位跳过整体校验。
		case strings.Contains(err.Error(), "重复"), strings.Contains(err.Error(), "不得补班"),
			strings.Contains(err.Error(), "不在 off"):
			// 输入自身矛盾（节日日期重复、节日当天补班或节日当天为工作日
			// 却不在放假区间内），均由人工核对修正。
		default:
			t.Fatalf("selfCheck 意外失败: %v", err)
		}

		// 自检通过（无 TODO）时，render 产物必须能被根包原样加载。
		if !hasTODO && err == nil {
			path := filepath.Join(t.TempDir(), fmt.Sprintf("%04d.toml", year))
			if werr := os.WriteFile(path, []byte(render(d, "fuzz")), 0o644); werr != nil {
				t.Fatalf("写出草稿失败: %v", werr)
			}
			if _, lerr := goliday.LoadYear(path); lerr != nil {
				t.Fatalf("自检通过的草稿无法被 LoadYear 加载: %v", lerr)
			}
		}
	})
}
