package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goliday"
)

// ann2026 模拟 2026 年假设方案的官方公告（对应 testdata/2026.toml 的元旦/春节/清明/劳动/国庆部分）。
const ann2026 = `一、元旦：1月1日至1月3日放假，共3天。
二、春节：2月15日至22日放假调休，共8天，2月17日（正月初一，星期二）为春节。2月14日（星期六）、2月28日（星期六）上班。
三、清明节：4月4日至6日放假，共3天（4月5日为清明）。
四、劳动节：5月1日至5日放假调休，共5天。5月9日（星期六）上班。
五、国庆节：10月1日至8日放假，共8天。10月10日（星期六）、10月11日（星期日）上班。`

var wantOff2026 = map[string]bool{
	"2026-01-01": true, "2026-01-02": true,
	"2026-02-16": true, "2026-02-17": true, "2026-02-18": true, "2026-02-19": true, "2026-02-20": true,
	"2026-04-06": true,
	"2026-05-01": true, "2026-05-04": true, "2026-05-05": true,
	"2026-10-01": true, "2026-10-02": true, "2026-10-05": true, "2026-10-06": true, "2026-10-07": true, "2026-10-08": true,
}

var wantWork2026 = map[string]bool{
	"2026-02-14": true, "2026-02-28": true, "2026-05-09": true, "2026-10-10": true, "2026-10-11": true,
}

var wantFest2026 = map[string]string{
	"元旦":  "2026-01-01",
	"春节":  "2026-02-17",
	"清明节": "2026-04-05",
	"劳动节": "2026-05-01",
	"国庆节": "2026-10-01",
}

func fmtSet(ts []time.Time) map[string]bool {
	m := make(map[string]bool, len(ts))
	for _, t := range ts {
		m[t.Format(dateLayoutTool)] = true
	}
	return m
}

func assertSameSet(t *testing.T, label string, got, want map[string]bool) {
	t.Helper()
	for k := range got {
		if !want[k] {
			t.Errorf("%s 含意外日期 %s", label, k)
		}
	}
	for k := range want {
		if !got[k] {
			t.Errorf("%s 缺少日期 %s", label, k)
		}
	}
}

func genFor(t *testing.T, year int, text string) (*parseResult, *draft) {
	t.Helper()
	res := parseAnnouncement(year, text)
	return res, buildDraft(year, res, lunarMarks(text))
}

func TestParseAnnouncement2026(t *testing.T) {
	res, d := genFor(t, 2026, ann2026)

	if len(res.warnings) != 0 {
		t.Errorf("样例公告不应产生警告，got %v", res.warnings)
	}
	assertSameSet(t, "off", fmtSet(d.off), wantOff2026)
	assertSameSet(t, "work", fmtSet(d.work), wantWork2026)

	if len(d.festivals) != len(wantFest2026) {
		t.Fatalf("festival 数量 = %d，期望 %d：%+v", len(d.festivals), len(wantFest2026), d.festivals)
	}
	for _, f := range d.festivals {
		want, ok := wantFest2026[f.Name]
		if !ok {
			t.Errorf("意外节日 %q", f.Name)
			continue
		}
		if f.Date != want {
			t.Errorf("节日 %s date = %s，期望 %s", f.Name, f.Date, want)
		}
	}
	if len(d.todos) != 0 {
		t.Errorf("不应有 TODO：%v", d.todos)
	}
}

func TestSelfCheckAndLoadYearRoundTrip(t *testing.T) {
	_, d := genFor(t, 2026, ann2026)
	if err := selfCheck(d); err != nil {
		t.Fatalf("自检失败: %v", err)
	}

	doc := render(d, "样例公告")
	if !strings.Contains(doc, "由 goliday-tool gen 生成，请人工核对") {
		t.Errorf("生成的文件缺少文件头提示注释")
	}
	// 写为 <year>.toml 以满足 LoadYear 的文件名年份校验。
	path := filepath.Join(t.TempDir(), "2026.toml")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := goliday.LoadYear(path)
	if err != nil {
		t.Fatalf("LoadYear 加载生成文件失败: %v", err)
	}
	if cfg.Year != 2026 || len(cfg.Festivals) != 5 || len(cfg.Adjust.Off) != 17 || len(cfg.Adjust.Work) != 5 {
		t.Errorf("加载结果不符：year=%d festivals=%d off=%d work=%d",
			cfg.Year, len(cfg.Festivals), len(cfg.Adjust.Off), len(cfg.Adjust.Work))
	}
}

func TestGenTODOPlaceholder(t *testing.T) {
	const text = `一、端午节：6月19日至21日放假，共3天。
二、中秋节：9月25日至27日放假，共3天。`
	res, d := genFor(t, 2026, text)

	if len(d.festivals) != 2 {
		t.Fatalf("festival = %+v", d.festivals)
	}
	for _, f := range d.festivals {
		if f.Date != dateTODO {
			t.Errorf("节日 %s 应为 TODO 占位，got %s", f.Name, f.Date)
		}
	}
	if !errors.Is(selfCheck(d), errFestivalTODO) {
		t.Errorf("selfCheck 应返回 errFestivalTODO，got %v", selfCheck(d))
	}

	doc := render(d, "stdin")
	if !strings.Contains(doc, `date = "TODO"`) {
		t.Errorf("输出应含 date = \"TODO\" 占位")
	}
	if !strings.Contains(doc, strings.Join(d.todos, "、")) {
		t.Errorf("文件头注释应列出待补全节日：%v", d.todos)
	}
	// 含 TODO 的文件按设计无法通过 LoadYear（validate 会失败，属预期）。
	path := filepath.Join(t.TempDir(), "2026.toml")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := goliday.LoadYear(path); err == nil {
		t.Errorf("含 TODO 占位的文件不应能加载")
	}
	// 解析层面：off 应剔除周末（06-20/21、09-26/27 为周末）。
	off := fmtSet(d.off)
	if !off["2026-06-19"] || !off["2026-09-25"] || len(off) != 2 {
		t.Errorf("off = %v，期望仅 2026-06-19 与 2026-09-25", off)
	}
	if len(res.entries) != 2 {
		t.Errorf("entries = %d，期望 2", len(res.entries))
	}
}

func TestParseChineseNumerals(t *testing.T) {
	res, d := genFor(t, 2026,
		"二、春节：二月十五日至二十二日放假调休，共8天。二月十四日（星期六）上班。")
	if len(res.entries) != 1 || res.entries[0].name != "春节" {
		t.Fatalf("entries = %+v", res.entries)
	}
	assertSameSet(t, "off", fmtSet(d.off), map[string]bool{
		"2026-02-16": true, "2026-02-17": true, "2026-02-18": true, "2026-02-19": true, "2026-02-20": true,
	})
	assertSameSet(t, "work", fmtSet(d.work), map[string]bool{"2026-02-14": true})
}

func TestParseWorkPhrases(t *testing.T) {
	cases := []struct {
		text string
		want map[string]bool
	}{
		{"10月10日（星期六）上班。", map[string]bool{"2026-10-10": true}},
		{"10月10日（星期六）调休上班。", map[string]bool{"2026-10-10": true}},
		{"10月10日补班。", map[string]bool{"2026-10-10": true}},
		{"1月24日（星期六）、2月8日（星期日）上班。", map[string]bool{"2026-01-24": true, "2026-02-08": true}},
		// 补班句与假期条目同段（逗号相连）时，仅取"上班"前的日期。
		{"国庆节：10月1日至8日放假，共8天，10月10日上班。", map[string]bool{"2026-10-10": true}},
		{"春节：2月15日至22日放假调休，共8天，2月14日上班。", map[string]bool{"2026-02-14": true}},
	}
	for _, c := range cases {
		_, d := genFor(t, 2026, c.text)
		got := fmtSet(d.work)
		assertSameSet(t, "work("+c.text+")", got, c.want)
		if len(got) != len(c.want) {
			t.Errorf("%q → work = %v，期望 %v", c.text, got, c.want)
		}
	}
}

func TestParseWarningsAndSkipRules(t *testing.T) {
	// 无法解析的行 → 警告。
	res, _ := genFor(t, 2026, "本通知自发布之日起执行。")
	if len(res.entries) != 0 || len(res.warnings) == 0 || !strings.Contains(res.warnings[0], "无法解析的行") {
		t.Errorf("entries=%d warnings=%v", len(res.entries), res.warnings)
	}

	// 补班日期非周末 → 跳过并告警。
	res, d := genFor(t, 2026, "一、元旦：1月1日放假1天。3月3日（星期二）上班。")
	if len(d.work) != 0 {
		t.Errorf("非周末补班应跳过，got %v", d.work)
	}
	if len(res.warnings) == 0 || !strings.Contains(strings.Join(res.warnings, ";"), "不是周末") {
		t.Errorf("应产生补班非周末警告，got %v", res.warnings)
	}

	// 不存在的日期（2月30日）→ 告警且不写入。
	res, d = genFor(t, 2026, "一、春节：2月15日至2月30日放假，共8天。")
	if len(d.off) != 0 || len(d.work) != 0 {
		t.Errorf("非法日期区间应整体跳过，got off=%v", d.off)
	}
	if !strings.Contains(strings.Join(res.warnings, ";"), "无效") {
		t.Errorf("应产生区间无效警告，got %v", res.warnings)
	}

	// 节日名无法识别 → 警告。
	res, _ = genFor(t, 2026, "一、腊八节：1月26日放假1天。")
	if len(res.warnings) == 0 || !strings.Contains(strings.Join(res.warnings, ";"), "无法确定节日名") {
		t.Errorf("未识别节日名应告警，got %v", res.warnings)
	}

	// 无日期的放假条目 → 警告。
	res, _ = genFor(t, 2026, "一、元旦：放假1天。")
	if len(res.warnings) == 0 || !strings.Contains(strings.Join(res.warnings, ";"), "未解析出假期日期") {
		t.Errorf("无日期条目应告警，got %v", res.warnings)
	}
}

func TestParseCNNum(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"一", 1}, {"九", 9}, {"十", 10}, {"十五", 15}, {"二十", 20},
		{"二十一", 21}, {"三十", 30}, {"三十一", 31},
	}
	for _, c := range cases {
		got, ok := parseCNNum(c.in)
		if !ok || got != c.want {
			t.Errorf("parseCNNum(%q) = %d,%v，期望 %d", c.in, got, ok, c.want)
		}
	}
	if _, ok := parseCNNum("百"); ok {
		t.Errorf("parseCNNum(\"百\") 不应成功")
	}
}

func TestExtractDates(t *testing.T) {
	cases := []struct {
		in   string
		want []dateMD
	}{
		{"2026年4月4日至6日", []dateMD{{4, 4}, {4, 6}}},
		{"10月1日至8日", []dateMD{{10, 1}, {10, 8}}},
		{"1月24日（星期六）、2月8日（星期日）", []dateMD{{1, 24}, {2, 8}}},
		{"一月一日", []dateMD{{1, 1}}},
	}
	for _, c := range cases {
		got := extractDates(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("extractDates(%q) = %v，期望 %v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("extractDates(%q)[%d] = %v，期望 %v", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	data, _ := io.ReadAll(r)
	return string(data)
}

func TestRunValidate(t *testing.T) {
	if rc := runValidate(nil); rc != 2 {
		t.Errorf("无参数退出码 = %d，期望 2", rc)
	}
	if rc := runValidate([]string{"../../testdata/2025.toml", "../../testdata/2026.toml"}); rc != 0 {
		t.Errorf("合法文件退出码 = %d，期望 0", rc)
	}
	if rc := runValidate([]string{"../../testdata/2026.toml", "../../testdata/invalid/dup.toml"}); rc != 1 {
		t.Errorf("含非法文件退出码 = %d，期望 1", rc)
	}
}

func TestValidateInvalidSamples(t *testing.T) {
	cases := []struct{ file, wantErr string }{
		{"off_weekend.toml", "周末"},
		{"dup.toml", "重复"},
		{"work_weekday.toml", "工作日"},
		{"invalid_date.toml", "非法日期"},
		{"year_mismatch.toml", "不一致"},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "invalid", c.file))
			if err != nil {
				t.Fatal(err)
			}
			// LoadYear 校验文件名年份，故复制为临时 2026.toml，使内容违规成为失败原因。
			path := filepath.Join(t.TempDir(), "2026.toml")
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := goliday.LoadYear(path); err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("LoadYear 错误 = %v，期望包含 %q", err, c.wantErr)
			}
			var rc int
			out := captureStdout(t, func() { rc = runValidate([]string{path}) })
			if rc != 1 || !strings.HasPrefix(strings.TrimSpace(out), "FAIL "+path) {
				t.Fatalf("runValidate rc=%d 输出 %q", rc, out)
			}
		})
	}
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunGenFromFile(t *testing.T) {
	ann := writeTempFile(t, "notice.txt", ann2026)
	out := filepath.Join(t.TempDir(), "2026.toml")
	if rc := runGen([]string{"-year", "2026", "-file", ann, "-out", out}); rc != 0 {
		t.Fatalf("runGen 退出码 = %d，期望 0", rc)
	}
	if _, err := goliday.LoadYear(out); err != nil {
		t.Fatalf("生成的文件应可被 LoadYear 加载: %v", err)
	}
}

func TestRunGenFromStdin(t *testing.T) {
	ann := writeTempFile(t, "notice.txt", ann2026)
	old := os.Stdin
	f, err := os.Open(ann)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = f
	defer func() { os.Stdin = old; f.Close() }()

	out := filepath.Join(t.TempDir(), "2026.toml")
	if rc := runGen([]string{"-year", "2026", "-out", out}); rc != 0 {
		t.Fatalf("runGen（stdin）退出码 = %d，期望 0", rc)
	}
	if _, err := goliday.LoadYear(out); err != nil {
		t.Fatalf("生成的文件应可被 LoadYear 加载: %v", err)
	}
}

func TestRunGenUsageErrors(t *testing.T) {
	if rc := runGen(nil); rc != 2 {
		t.Errorf("缺 -year/-out 退出码 = %d，期望 2", rc)
	}
	if rc := runGen([]string{"-year", "2026"}); rc != 2 {
		t.Errorf("缺 -out 退出码 = %d，期望 2", rc)
	}
	if rc := runGen([]string{"-year", "abc", "-out", "-"}); rc != 2 {
		t.Errorf("非法 -year 退出码 = %d，期望 2", rc)
	}
}

func TestRunGenFileErrors(t *testing.T) {
	if rc := runGen([]string{"-year", "2026", "-file", "/nonexistent/notice.txt", "-out", "-"}); rc != 1 {
		t.Errorf("公告文件不存在退出码 = %d，期望 1", rc)
	}
	if rc := runGen([]string{"-year", "2026", "-file", writeTempFile(t, "empty.txt", "本通知自发布之日起执行。"),
		"-out", filepath.Join(t.TempDir(), "2026.toml")}); rc != 1 {
		t.Errorf("无可解析条目退出码 = %d，期望 1", rc)
	}
}
