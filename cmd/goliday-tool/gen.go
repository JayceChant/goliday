package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/JayceChant/goliday"
)

// dateLayoutTool 与根包一致的 YYYY-MM-DD 日期格式。
const dateLayoutTool = "2006-01-02"

// dateTODO 无法推断节日当天日期时的占位值。
const dateTODO = "TODO"

// errFestivalTODO 表示草稿中存在未补全的 festival 日期，跳过整体校验。
var errFestivalTODO = errors.New("存在未补全的 festival 日期（TODO 占位）")

// dateMD 公告中的"月-日"日期；年份统一取 -year 参数，公告中的年份前缀被忽略。
type dateMD struct {
	month, day int
}

// festivalEntry 公告中识别到的一个节日条目：规范化名称与假期区间（含两端）。
type festivalEntry struct {
	name       string
	start, end time.Time
}

// parseResult 公告解析结果。
type parseResult struct {
	entries  []festivalEntry
	work     []time.Time // 补班候选日期
	warnings []string
}

func (r *parseResult) warnf(format string, args ...any) {
	r.warnings = append(r.warnings, fmt.Sprintf(format, args...))
}

// hasEntry 报告某规范化节日名是否已被收录（用于忽略重复条目）。
func (r *parseResult) hasEntry(name string) bool {
	for _, e := range r.entries {
		if e.name == name {
			return true
		}
	}
	return false
}

// draftFestival 生成结果中的节日，Date 为 YYYY-MM-DD 或占位 TODO。
type draftFestival struct {
	Name string
	Date string
}

// draft 由公告生成的稀疏配置草稿。
type draft struct {
	year      int
	festivals []draftFestival
	off, work []time.Time
	todos     []string
}

// mdPattern 匹配"X月X日"：月/日支持阿拉伯数字与中文数字（一月一日、二十一日），
// 月份部分可整体省略（"4月4日至6日"的"6日"，解析时沿用最近出现的月份），
// 日期前可有"X年"前缀（由 dateRe 处理，解析时忽略年份）。
const mdPattern = `(?:(?:(\d{1,2})|([一二三四五六七八九十]{1,2}))月)?(?:(\d{1,2})|([一二三四五六七八九十]{1,3}))日`

var (
	// dateRe 从文本中提取全部"X月X日"。
	dateRe = regexp.MustCompile(`(?:\d{2,4}年)?` + mdPattern)
	// workRe 匹配"X月X日（星期X）、……（调休/补班）上班"补班句式，捕获日期部分。
	workRe = regexp.MustCompile(
		`((?:` + mdPattern + `)(?:\s*[（(][^（）()]*[）)])?` +
			`(?:\s*[、，,]\s*(?:` + mdPattern + `)(?:\s*[（(][^（）()]*[）)])?)*)` +
			`\s*(?:需要|应当)?(?:调休)?(?:补班|上班)`)
	// markAfterDateRe 匹配"X月X日（……正月初一/除夕/八月十五/清明……）"表述。
	markAfterDateRe = regexp.MustCompile(`(\d{1,2})月(\d{1,2})日\s*[（(][^（）()]*?(正月初一|除夕|八月十五|清明)`)
	// markInParenRe 匹配"（……X月X日……正月初一……）"表述（日期与农历/名称同括号内）。
	markInParenRe = regexp.MustCompile(`[（(][^（）()]*?(\d{1,2})月(\d{1,2})日[^（）()]*?(正月初一|除夕|八月十五|清明)[^（）()]*[）)]`)
)

// lunarMarkerNames 农历/名称表述 → 规范化节日名。
var lunarMarkerNames = map[string]string{
	"正月初一": "春节",
	"除夕":   "除夕",
	"八月十五": "中秋节",
	"清明":   "清明节",
}

// fixedFestivalDates 公历固定日期节日。
var fixedFestivalDates = map[string]dateMD{
	"元旦":  {1, 1},
	"劳动节": {5, 1},
	"国庆节": {10, 1},
}

// nameAliases 节日名容错别名表（同一规范名的长别名在前）。
var nameAliases = []struct{ alias, name string }{
	{"元旦", "元旦"},
	{"春节", "春节"},
	{"清明节", "清明节"}, {"清明", "清明节"},
	{"五一国际劳动节", "劳动节"}, {"五一劳动节", "劳动节"}, {"劳动节", "劳动节"}, {"五一节", "劳动节"}, {"五一", "劳动节"},
	{"端午节", "端午节"}, {"端午", "端午节"},
	{"中秋节", "中秋节"}, {"中秋", "中秋节"},
	{"国庆节", "国庆节"}, {"国庆", "国庆节"}, {"十一", "国庆节"},
	{"除夕", "除夕"},
}

var cnDigits = map[rune]int{'一': 1, '二': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}

// parseCNNum 解析一~三十一范围内的中文数字。
func parseCNNum(s string) (int, bool) {
	rs := []rune(s)
	ten := -1
	for i, r := range rs {
		if r == '十' {
			ten = i
			break
		}
	}
	if ten < 0 {
		if len(rs) != 1 {
			return 0, false
		}
		v, ok := cnDigits[rs[0]]
		return v, ok
	}
	hi := 1
	if ten > 0 {
		if ten != 1 {
			return 0, false
		}
		v, ok := cnDigits[rs[0]]
		if !ok {
			return 0, false
		}
		hi = v
	}
	lo := 0
	if ten != len(rs)-1 {
		if len(rs)-ten != 2 {
			return 0, false
		}
		v, ok := cnDigits[rs[len(rs)-1]]
		if !ok {
			return 0, false
		}
		lo = v
	}
	return hi*10 + lo, true
}

// splitSegments 按行、句号与分号切分公告文本为条目片段。
func splitSegments(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '\r' || r == '。' || r == '；' || r == ';'
	})
}

// extractDates 提取文本中的全部"X月X日"；省略月份的日期沿用最近出现的月份（跨月简写）。
func extractDates(s string) []dateMD {
	var out []dateMD
	month := 0
	for _, m := range dateRe.FindAllStringSubmatch(s, -1) {
		mo, day := 0, 0
		var ok bool
		switch {
		case m[1] != "":
			mo, _ = strconv.Atoi(m[1])
		case m[2] != "":
			mo, ok = parseCNNum(m[2])
			if !ok {
				continue
			}
		}
		switch {
		case m[3] != "":
			day, _ = strconv.Atoi(m[3])
		case m[4] != "":
			day, ok = parseCNNum(m[4])
			if !ok {
				continue
			}
		}
		if mo == 0 {
			mo = month
		}
		if mo == 0 || day == 0 {
			continue // 无从推断月份，放弃该日期
		}
		month = mo
		out = append(out, dateMD{mo, day})
	}
	return out
}

// canonicalNames 从条目文本（取"："前的节日名部分）识别规范化节日名，容错别名。
func canonicalNames(s string) []string {
	if idx := strings.IndexAny(s, "：:"); idx > 0 {
		s = s[:idx]
	}
	var out []string
	seen := map[string]bool{}
	for _, a := range nameAliases {
		matched := strings.Contains(s, a.alias)
		if a.alias == "十一" {
			matched = matched && !strings.Contains(s, "十一月") // 排除"十一月"误报
		}
		if matched && !seen[a.name] {
			seen[a.name] = true
			out = append(out, a.name)
		}
	}
	return out
}

// mkDate 将月-日与年份组合为 time.Time，校验日期真实存在（如 2 月 30 日报无效）。
func mkDate(year int, d dateMD) (time.Time, bool) {
	t := time.Date(year, time.Month(d.month), d.day, 0, 0, 0, 0, time.UTC)
	if int(t.Month()) != d.month || t.Day() != d.day {
		return time.Time{}, false
	}
	return t, true
}

// weekdayName 返回中文星期名。
func weekdayName(t time.Time) string {
	names := [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	return names[int(t.Weekday())]
}

// lunarMarks 扫描全文，收集"X月X日（正月初一/除夕/八月十五/清明）"类表述，
// 返回规范化节日名 → 月-日 映射（同一节日取首次出现）。
func lunarMarks(text string) map[string]dateMD {
	marks := make(map[string]dateMD)
	for _, re := range []*regexp.Regexp{markAfterDateRe, markInParenRe} {
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			name := lunarMarkerNames[m[3]]
			if _, dup := marks[name]; dup {
				continue
			}
			mo, err1 := strconv.Atoi(m[1])
			day, err2 := strconv.Atoi(m[2])
			if err1 != nil || err2 != nil {
				continue
			}
			marks[name] = dateMD{mo, day}
		}
	}
	return marks
}

// parseAnnouncement 解析公告文本，识别"节日名：日期段 放假"条目与"X月X日（星期X）上班"补班句式。
// 无法识别的非空片段记入 warnings。
func parseAnnouncement(year int, text string) *parseResult {
	res := &parseResult{}
	for _, seg := range splitSegments(text) {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		matched := false

		// 补班句式（一段内可出现多条）。
		for _, m := range workRe.FindAllStringSubmatch(seg, -1) {
			matched = true
			for _, d := range extractDates(m[1]) {
				t, ok := mkDate(year, d)
				if !ok {
					res.warnf("忽略不存在的日期 %d月%d日", d.month, d.day)
					continue
				}
				res.work = append(res.work, t)
			}
		}

		// 放假条目：节日名与日期取"放假"之前的片段。
		if idx := strings.Index(seg, "放假"); idx >= 0 {
			matched = true
			window := seg[:idx]
			names := canonicalNames(window)
			dates := extractDates(window)
			switch {
			case len(names) == 0:
				res.warnf("识别到放假安排但无法确定节日名：%s", seg)
			case len(dates) == 0:
				res.warnf("识别到 %s 但未解析出假期日期：%s", strings.Join(names, "、"), seg)
			default:
				start, ok1 := mkDate(year, dates[0])
				end, ok2 := mkDate(year, dates[len(dates)-1])
				if !ok1 || !ok2 || end.Before(start) {
					res.warnf("假期区间无效：%s", seg)
				} else {
					for _, n := range names {
						if res.hasEntry(n) {
							res.warnf("忽略重复的 %s 条目", n)
							continue
						}
						res.entries = append(res.entries, festivalEntry{name: n, start: start, end: end})
					}
				}
			}
		}

		if !matched {
			res.warnf("无法解析的行：%s", seg)
		}
	}
	return res
}

// inferFestivalDate 推断节日当天日期：农历/名称表述优先，其次公历固定节日，
// 清明节再退而取四月假期首日；无法推断返回 false。
func inferFestivalDate(e festivalEntry, marks map[string]dateMD) (dateMD, bool) {
	if md, ok := marks[e.name]; ok {
		return md, true
	}
	if md, ok := fixedFestivalDates[e.name]; ok {
		return md, true
	}
	if e.name == "清明节" && e.start.Month() == time.April {
		return dateMD{int(e.start.Month()), e.start.Day()}, true
	}
	return dateMD{}, false
}

// buildDraft 将解析结果组装为稀疏配置草稿：
//   - 假期区间逐日展开，周六/周日不写入 off，周一~五写入 off；
//   - 补班日期校验为周六/日后写入 work，非周末的跳过并告警；
//   - festival.date 按推断规则得出，无法推断时置 TODO 并记录待补全名单。
//
// 组装过程中发现的问题追加到 res.warnings。
func buildDraft(year int, res *parseResult, marks map[string]dateMD) *draft {
	d := &draft{year: year}

	seenOff := map[time.Time]bool{}
	for _, e := range res.entries {
		for cur := e.start; !cur.After(e.end); cur = cur.AddDate(0, 0, 1) {
			if wd := cur.Weekday(); wd == time.Saturday || wd == time.Sunday {
				continue // 自然周末不写入稀疏表
			}
			if seenOff[cur] {
				res.warnf("off 日期 %s 在多个条目中重复，已去重", cur.Format(dateLayoutTool))
				continue
			}
			seenOff[cur] = true
			d.off = append(d.off, cur)
		}
	}

	seenWork := map[time.Time]bool{}
	for _, t := range res.work {
		if wd := t.Weekday(); wd != time.Saturday && wd != time.Sunday {
			res.warnf("补班日期 %s（%s）不是周末，已跳过", t.Format(dateLayoutTool), weekdayName(t))
			continue
		}
		if seenWork[t] {
			continue
		}
		seenWork[t] = true
		d.work = append(d.work, t)
	}

	for _, e := range res.entries {
		dateStr := ""
		if md, ok := inferFestivalDate(e, marks); ok {
			if t, okT := mkDate(year, md); okT {
				dateStr = t.Format(dateLayoutTool)
			}
		}
		if dateStr == "" {
			dateStr = dateTODO
			d.todos = append(d.todos, e.name)
			res.warnf("无法推断 %s 的节日当天日期，以 TODO 占位，请人工补全", e.name)
		}
		d.festivals = append(d.festivals, draftFestival{Name: e.name, Date: dateStr})
	}

	sort.Slice(d.off, func(i, j int) bool { return d.off[i].Before(d.off[j]) })
	sort.Slice(d.work, func(i, j int) bool { return d.work[i].Before(d.work[j]) })
	sort.SliceStable(d.festivals, func(i, j int) bool {
		a, b := d.festivals[i], d.festivals[j]
		if (a.Date == dateTODO) != (b.Date == dateTODO) {
			return b.Date == dateTODO // TODO 排在末尾
		}
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		return a.Name < b.Name
	})
	return d
}

// render 将草稿渲染为 TOML 文本。
func render(d *draft, source string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 来源：%d 年节假日安排公告（%s）；由 goliday-tool gen 生成，请人工核对后再入库。\n", d.year, source)
	b.WriteString("# 稀疏表原则：off 仅记录周一~周五的调休放假日，work 仅记录周六/周日的补班日；假期内的自然周末日不写入。\n")
	if len(d.todos) > 0 {
		fmt.Fprintf(&b, "# TODO：以下节日的当天日期无法自动推断，请人工补全（当前为占位 %q，此时 validate 会失败，属预期）：%s。\n",
			dateTODO, strings.Join(d.todos, "、"))
	}
	fmt.Fprintf(&b, "\nyear = %d\n", d.year)
	for _, f := range d.festivals {
		b.WriteString("\n[[festival]]\n")
		fmt.Fprintf(&b, "name = %q\n", f.Name)
		fmt.Fprintf(&b, "date = %q\n", f.Date)
	}
	b.WriteString("\n[adjust]\n")
	writeDates(&b, "off", d.off)
	writeDates(&b, "work", d.work)
	return b.String()
}

// writeDates 按每行 4 个日期写出 TOML 数组。
func writeDates(b *strings.Builder, key string, ds []time.Time) {
	if len(ds) == 0 {
		fmt.Fprintf(b, "%s = []\n", key)
		return
	}
	fmt.Fprintf(b, "%s = [\n", key)
	for i, t := range ds {
		if i%4 == 0 {
			b.WriteString("  ")
		}
		fmt.Fprintf(b, "%q", t.Format(dateLayoutTool))
		switch {
		case i == len(ds)-1:
			b.WriteString("\n")
		case i%4 == 3:
			b.WriteString(",\n")
		default:
			b.WriteString(", ")
		}
	}
	b.WriteString("]\n")
}

// selfCheck 输出前自检：构造 YearConfig 调用根包校验逻辑。
// 存在 TODO 占位时返回 errFestivalTODO，调用方跳过整体校验仅告警。
func selfCheck(d *draft) error {
	cfg := &goliday.YearConfig{Year: d.year}
	for _, f := range d.festivals {
		if f.Date == dateTODO {
			return errFestivalTODO
		}
		t, err := time.Parse(dateLayoutTool, f.Date)
		if err != nil {
			return err
		}
		cfg.Festivals = append(cfg.Festivals, goliday.Festival{Name: f.Name, Date: t})
	}
	cfg.Adjust = goliday.Adjust{Off: d.off, Work: d.work}
	return cfg.Validate()
}

// writeOut 写出内容，path 为 "-" 时写到 stdout。
func writeOut(path, content string) error {
	if path == "-" {
		_, err := io.WriteString(os.Stdout, content)
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// runGen 实现 gen 子命令：读取公告文本，解析并生成稀疏 TOML 草稿，返回退出码。
func runGen(args []string) int {
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	fs.Usage = usage
	year := fs.Int("year", 0, "年份（必填，如 2027）")
	out := fs.String("out", "", "输出文件路径（必填），\"-\" 表示 stdout")
	file := fs.String("file", "", "公告文本文件路径，缺省或 \"-\" 表示 stdin")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *year <= 0 || *year > 9999 || *out == "" {
		fmt.Fprintln(os.Stderr, "错误：-year 与 -out 为必填参数")
		usage()
		return 2
	}

	source := "stdin"
	var r io.Reader = os.Stdin
	if *file != "" && *file != "-" {
		f, err := os.Open(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误：读取公告文件失败: %v\n", err)
			return 1
		}
		// 只读文件，Close 错误无需处理。
		defer func() { _ = f.Close() }()
		r = f
		source = *file
	}
	data, err := io.ReadAll(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：读取公告内容失败: %v\n", err)
		return 1
	}

	res := parseAnnouncement(*year, string(data))
	if len(res.entries) == 0 {
		for _, w := range res.warnings {
			fmt.Fprintf(os.Stderr, "警告：%s\n", w)
		}
		fmt.Fprintln(os.Stderr, "错误：未能从公告中解析出任何放假条目")
		return 1
	}
	d := buildDraft(*year, res, lunarMarks(string(data)))
	for _, w := range res.warnings {
		fmt.Fprintf(os.Stderr, "警告：%s\n", w)
	}

	rc := 0
	if err := selfCheck(d); err != nil {
		if errors.Is(err, errFestivalTODO) {
			fmt.Fprintf(os.Stderr, "警告：%v，已跳过整体校验；请人工补全后再 validate\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "错误：自检失败：%v\n", err)
			rc = 1
		}
	}

	if err := writeOut(*out, render(d, source)); err != nil {
		fmt.Fprintf(os.Stderr, "错误：写出文件失败: %v\n", err)
		return 1
	}
	outDesc := *out
	if outDesc == "-" {
		outDesc = "stdout"
	}
	fmt.Printf("已生成 %s：%d 个节日，off %d 天，work %d 天（请人工核对）\n",
		outDesc, len(d.festivals), len(d.off), len(d.work))
	return rc
}
