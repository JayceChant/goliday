// 白盒测试（package main）：经未导出的 newHandler 构造完整处理链，
// 覆盖全部 HTTP API Scenario。
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/JayceChant/goliday"
)

// newTestHandler 从 ../../testdata 加载配置，构造与 main 一致的处理链。
func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	store, err := goliday.LoadDir("../../testdata")
	if err != nil {
		t.Fatalf("加载测试配置失败: %v", err)
	}
	return newHandler(store, goliday.NewCalendar(store))
}

// doRequest 发送请求并断言响应体为合法 JSON 对象，返回状态码与解析结果。
func doRequest(t *testing.T, h http.Handler, method, target string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s %s 响应不是合法 JSON（状态码 %d）: %v\n响应体: %s", method, target, rec.Code, err, rec.Body.String())
	}
	return rec.Code, body
}

// wantNum 断言 JSON 数值字段（解析为 float64）。
func wantNum(t *testing.T, name string, got any, want float64) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %#v, want %v", name, got, want)
	}
}

// wantStr 断言 JSON 字符串字段。
func wantStr(t *testing.T, name string, got any, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %#v, want %q", name, got, want)
	}
}

// dayAt 取响应 days 数组的第 i 个元素。
func dayAt(t *testing.T, body map[string]any, i int) map[string]any {
	t.Helper()
	days, ok := body["days"].([]any)
	if !ok {
		t.Fatalf("响应缺少 days 数组: %#v", body["days"])
	}
	if i >= len(days) {
		t.Fatalf("days 长度为 %d，越界访问 %d", len(days), i)
	}
	d, ok := days[i].(map[string]any)
	if !ok {
		t.Fatalf("days[%d] 不是 JSON 对象: %#v", i, days[i])
	}
	return d
}

// statsOf 取响应 stats 对象。
func statsOf(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	st, ok := body["stats"].(map[string]any)
	if !ok {
		t.Fatalf("响应缺少 stats 对象: %#v", body["stats"])
	}
	return st
}

// 1. 单日粗粒度。
func TestSingleDayCoarse(t *testing.T) {
	h := newTestHandler(t)
	code, body := doRequest(t, h, http.MethodGet, "/api/v1/days?date=2026-02-20")
	if code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", code)
	}
	wantStr(t, "date", body["date"], "2026-02-20")
	wantNum(t, "type", body["type"], 2)
	wantStr(t, "type_label", body["type_label"], "rest")
}

// 2. 单日细粒度。
func TestSingleDayDetailed(t *testing.T) {
	h := newTestHandler(t)
	cases := []struct {
		date      string
		wantType  float64
		wantLabel string
	}{
		{"2026-02-20", 10, "rest|adjusted"},
		{"2026-02-17", 6, "rest|festival"},
		{"2026-02-28", 17, "work|compensate"},
		{"2026-04-05", 6, "rest|festival"},
	}
	for _, c := range cases {
		code, body := doRequest(t, h, http.MethodGet, "/api/v1/days?date="+c.date+"&detailed=true")
		if code != http.StatusOK {
			t.Fatalf("date=%s 状态码 = %d, want 200", c.date, code)
		}
		wantStr(t, "date("+c.date+")", body["date"], c.date)
		wantNum(t, "type("+c.date+")", body["type"], c.wantType)
		wantStr(t, "type_label("+c.date+")", body["type_label"], c.wantLabel)
	}
}

// 3. 区间查询与粗粒度统计。
func TestRangeQuery(t *testing.T) {
	h := newTestHandler(t)
	code, body := doRequest(t, h, http.MethodGet, "/api/v1/days?start=2026-02-14&end=2026-02-17")
	if code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", code)
	}
	wantStr(t, "mode", body["mode"], "range")
	wantStr(t, "start", body["start"], "2026-02-14")
	wantStr(t, "end", body["end"], "2026-02-17")
	wantNum(t, "total_days", body["total_days"], 3)

	d0 := dayAt(t, body, 0)
	wantStr(t, "days[0].date", d0["date"], "2026-02-14")
	wantNum(t, "days[0].type", d0["type"], 17) // 02-14 补班：Work|Compensate
	d2 := dayAt(t, body, 2)
	wantStr(t, "days[2].date", d2["date"], "2026-02-16")
	wantNum(t, "days[2].type", d2["type"], 10) // 02-16 调休：Rest|Adjusted

	// detailed 默认 false：粗粒度统计。
	st := statsOf(t, body)
	wantNum(t, "stats.workday", st["workday"], 1) // 02-14 补班归上班日
	wantNum(t, "stats.holiday", st["holiday"], 2) // 02-15 周末、02-16 调休
}

// 4. 离散日期列表：去重升序。
func TestDatesListDedupSorted(t *testing.T) {
	h := newTestHandler(t)
	code, body := doRequest(t, h, http.MethodGet,
		"/api/v1/days?dates=2026-02-16,2026-02-28,2026-02-17,2026-02-16")
	if code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", code)
	}
	wantStr(t, "mode", body["mode"], "list")
	wantNum(t, "total_days", body["total_days"], 3)
	want := []string{"2026-02-16", "2026-02-17", "2026-02-28"}
	days, ok := body["days"].([]any)
	if !ok {
		t.Fatalf("响应缺少 days 数组: %#v", body["days"])
	}
	if len(days) != len(want) {
		t.Fatalf("days 长度 = %d, want %d: %#v", len(days), len(want), days)
	}
	for i, w := range want {
		wantStr(t, fmt.Sprintf("days[%d].date", i), dayAt(t, body, i)["date"], w)
	}
}

// 5. 区间与列表混合：并集。
func TestMixedRangeAndDates(t *testing.T) {
	h := newTestHandler(t)
	code, body := doRequest(t, h, http.MethodGet,
		"/api/v1/days?start=2026-02-01&end=2026-02-03&dates=2026-03-08")
	if code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", code)
	}
	wantStr(t, "mode", body["mode"], "list")
	wantNum(t, "total_days", body["total_days"], 3)
	want := []string{"2026-02-01", "2026-02-02", "2026-03-08"}
	for i, w := range want {
		wantStr(t, fmt.Sprintf("days[%d].date", i), dayAt(t, body, i)["date"], w)
	}
}

// 6. 细粒度统计五键 MECE：各计一类日，之和等于总天数。
func TestFineStatsMECE(t *testing.T) {
	h := newTestHandler(t)
	code, body := doRequest(t, h, http.MethodGet,
		"/api/v1/days?dates=2026-02-17,2026-02-28&detailed=true")
	if code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", code)
	}
	st := statsOf(t, body)
	wantNum(t, "stats.festival", st["festival"], 1)
	wantNum(t, "stats.compensate", st["compensate"], 1)
	wantNum(t, "stats.adjusted", st["adjusted"], 0)
	wantNum(t, "stats.weekend", st["weekend"], 0)
	wantNum(t, "stats.ordinary", st["ordinary"], 0)
}

// 7. stats 与 days 接口口径一致，且 stats 响应无 days 字段。
func TestStatsMatchesDays(t *testing.T) {
	h := newTestHandler(t)
	query := "start=2026-02-14&end=2026-02-17"
	_, daysBody := doRequest(t, h, http.MethodGet, "/api/v1/days?"+query)
	code, statsBody := doRequest(t, h, http.MethodGet, "/api/v1/stats?"+query)
	if code != http.StatusOK {
		t.Fatalf("stats 状态码 = %d, want 200", code)
	}
	if !reflect.DeepEqual(daysBody["stats"], statsBody["stats"]) {
		t.Errorf("stats 不一致: days 接口 %#v, stats 接口 %#v", daysBody["stats"], statsBody["stats"])
	}
	if daysBody["total_days"] != statsBody["total_days"] {
		t.Errorf("total_days 不一致: days 接口 %#v, stats 接口 %#v",
			daysBody["total_days"], statsBody["total_days"])
	}
	if _, ok := statsBody["days"]; ok {
		t.Errorf("stats 响应不应包含 days 字段: %#v", statsBody["days"])
	}
}

// 8. 参数错误统一 400。
func TestBadRequest(t *testing.T) {
	h := newTestHandler(t)
	cases := []struct {
		target   string
		wantCode string
	}{
		{"/api/v1/days?date=2026-02-30", "invalid_date"},
		{"/api/v1/days?start=2026-03-05&end=2026-03-01", "invalid_range"},
		{"/api/v1/days?start=2026-01-01&end=2027-06-01", "invalid_range"},
		{"/api/v1/days", "missing_query"},
		{"/api/v1/days?dates=abc", "invalid_date"},
		{"/api/v1/days?start=abc&end=2026-03-05", "invalid_date"},
		{"/api/v1/days?start=2026-03-05&end=abc", "invalid_date"},
	}
	for _, c := range cases {
		code, body := doRequest(t, h, http.MethodGet, c.target)
		if code != http.StatusBadRequest {
			t.Errorf("%s 状态码 = %d, want 400", c.target, code)
		}
		errObj, ok := body["error"].(map[string]any)
		if !ok {
			t.Errorf("%s 响应缺少 error 对象: %v", c.target, body)
			continue
		}
		wantStr(t, "error.code("+c.target+")", errObj["code"], c.wantCode)
	}
}

// 9. 健康检查与 404/405。
func TestHealthzAndRouting(t *testing.T) {
	h := newTestHandler(t)

	code, body := doRequest(t, h, http.MethodGet, "/healthz")
	if code != http.StatusOK {
		t.Fatalf("healthz 状态码 = %d, want 200", code)
	}
	wantStr(t, "status", body["status"], "ok")
	years, ok := body["years"].([]any)
	if !ok {
		t.Fatalf("healthz 响应缺少 years 数组: %#v", body["years"])
	}
	got := make(map[float64]bool, len(years))
	for _, y := range years {
		if f, ok := y.(float64); ok {
			got[f] = true
		}
	}
	for _, want := range []float64{2025, 2026} {
		if !got[want] {
			t.Errorf("years 缺少 %v: %v", want, years)
		}
	}

	code, body = doRequest(t, h, http.MethodGet, "/api/v1/nope")
	if code != http.StatusNotFound {
		t.Errorf("未匹配路径状态码 = %d, want 404", code)
	}
	if _, ok := body["error"].(map[string]any); !ok {
		t.Errorf("404 响应应为 JSON 错误格式: %v", body)
	}

	code, body = doRequest(t, h, http.MethodPost, "/api/v1/days")
	if code != http.StatusMethodNotAllowed {
		t.Errorf("方法不符状态码 = %d, want 405", code)
	}
	if _, ok := body["error"].(map[string]any); !ok {
		t.Errorf("405 响应应为 JSON 错误格式: %v", body)
	}
}

// 10. 未加载年份：单日/区间/离散/混合均 400 year_not_loaded，
// message 列出升序去重的未加载年份；不再回退周休判断。
func TestYearNotLoaded(t *testing.T) {
	h := newTestHandler(t)
	cases := []struct {
		target   string
		wantMsgs []string
	}{
		{"/api/v1/days?date=2027-05-01", []string{"2027"}},
		{"/api/v1/stats?date=2027-05-01", []string{"2027"}},
		{"/api/v1/days?dates=2026-02-17,2027-03-01,2028-04-01", []string{"2027", "2028"}},
		// 区间含未加载中间年（跨度 ≤366，days 亦触发年份校验）。
		{"/api/v1/days?start=2026-12-20&end=2027-01-05", []string{"2027"}},
		// 混合并集：区间 + 列表含未加载年。
		{"/api/v1/days?start=2026-02-01&end=2026-02-03&dates=2027-03-08", []string{"2027"}},
	}
	for _, c := range cases {
		code, body := doRequest(t, h, http.MethodGet, c.target)
		if code != http.StatusBadRequest {
			t.Errorf("%s 状态码 = %d, want 400", c.target, code)
			continue
		}
		errObj, ok := body["error"].(map[string]any)
		if !ok {
			t.Errorf("%s 响应缺少 error 对象: %v", c.target, body)
			continue
		}
		wantStr(t, "error.code("+c.target+")", errObj["code"], "year_not_loaded")
		msg, _ := errObj["message"].(string)
		for _, want := range c.wantMsgs {
			if !strings.Contains(msg, want) {
				t.Errorf("%s message = %q, 应包含 %q", c.target, msg, want)
			}
		}
	}
}

// 11. stats 不限跨度（前缀和统计）：跨 2025→2026 大区间成功且等于
// 分段统计之和；days 同区间仍被 366 上限拒绝；空区间 200 全零。
func TestStatsLargeRangeAndEmptyRange(t *testing.T) {
	h := newTestHandler(t)

	code, body := doRequest(t, h, http.MethodGet, "/api/v1/stats?start=2025-01-01&end=2027-01-01")
	if code != http.StatusOK {
		t.Fatalf("stats 大跨度状态码 = %d, want 200（body: %v）", code, body)
	}
	wantNum(t, "total_days", body["total_days"], 730)
	got := statsOf(t, body)
	wantNum(t, "stats.workday+holiday 之和",
		got["workday"].(float64)+got["holiday"].(float64), 730)

	// 分段对照：2025 全年 + 2026 全年。
	_, y2025 := doRequest(t, h, http.MethodGet, "/api/v1/stats?start=2025-01-01&end=2026-01-01")
	_, y2026 := doRequest(t, h, http.MethodGet, "/api/v1/stats?start=2026-01-01&end=2027-01-01")
	s25, s26 := statsOf(t, y2025), statsOf(t, y2026)
	wantNum(t, "分段 workday 之和",
		got["workday"].(float64), s25["workday"].(float64)+s26["workday"].(float64))
	wantNum(t, "分段 holiday 之和",
		got["holiday"].(float64), s25["holiday"].(float64)+s26["holiday"].(float64))

	// days 明细同区间：超 366 天被拒。
	code, body = doRequest(t, h, http.MethodGet, "/api/v1/days?start=2025-01-01&end=2027-01-01")
	if code != http.StatusBadRequest {
		t.Fatalf("days 大跨度状态码 = %d, want 400", code)
	}
	errObj, _ := body["error"].(map[string]any)
	wantStr(t, "days 大跨度 error.code", errObj["code"], "invalid_range")

	// 空区间（2027 未加载）：无覆盖年份，200 全零。
	code, body = doRequest(t, h, http.MethodGet, "/api/v1/stats?start=2027-01-01&end=2027-01-01")
	if code != http.StatusOK {
		t.Fatalf("stats 空区间状态码 = %d, want 200", code)
	}
	wantNum(t, "空区间 total_days", body["total_days"], 0)
	st := statsOf(t, body)
	wantNum(t, "空区间 workday", st["workday"], 0)
	wantNum(t, "空区间 holiday", st["holiday"], 0)
}

// 13. start/end 未成对（仅提供其一）返回 invalid_range。
func TestRangeUnpaired(t *testing.T) {
	h := newTestHandler(t)
	for _, target := range []string{
		"/api/v1/days?start=2026-03-05",
		"/api/v1/days?end=2026-03-05",
		"/api/v1/stats?start=2026-03-05",
	} {
		code, body := doRequest(t, h, http.MethodGet, target)
		if code != http.StatusBadRequest {
			t.Errorf("%s 状态码 = %d, want 400", target, code)
			continue
		}
		errObj, ok := body["error"].(map[string]any)
		if !ok {
			t.Errorf("%s 响应缺少 error 对象: %v", target, body)
			continue
		}
		wantStr(t, "error.code("+target+")", errObj["code"], "invalid_range")
	}
}

// 14. 混合查询中列表日期全部落在区间内：剔除后列表统计为空，
// total_days 即区间天数。
func TestMixedDatesAllInsideRange(t *testing.T) {
	h := newTestHandler(t)
	code, body := doRequest(t, h, http.MethodGet,
		"/api/v1/days?start=2026-02-01&end=2026-02-28&dates=2026-02-17,2026-02-20")
	if code != http.StatusOK {
		t.Fatalf("状态码 = %d, want 200", code)
	}
	wantStr(t, "mode", body["mode"], "list")
	wantNum(t, "total_days", body["total_days"], 27)
}

// 15. recover 中间件：下游 panic 统一转为 500 JSON 错误，不退出进程。
func TestRecoverMiddlewarePanic(t *testing.T) {
	h := recoverMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("boom")
	}))
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("状态码 = %d, want 500", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("响应缺少 error 对象: %v", body)
	}
	wantStr(t, "error.code", errObj["code"], "internal_error")
}

// 16. 日志中间件：下游未写任何响应时状态码兜底为 200。
func TestLoggingMiddlewareNoWrite(t *testing.T) {
	h := loggingMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/empty", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("状态码 = %d, want 200", rec.Code)
	}
}

// 17. statusRecorder：未先 WriteHeader 直接 Write 时状态码兜底为 200。
func TestStatusRecorderWriteDefaultStatus(t *testing.T) {
	rec := &statusRecorder{ResponseWriter: httptest.NewRecorder()}
	n, err := rec.Write([]byte("ok"))
	if err != nil || n != 2 {
		t.Fatalf("Write = %d, %v, want 2, nil", n, err)
	}
	if rec.status != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.status)
	}
}

// 12. 混合模式下 days 与 stats 口径一致（前缀和路径统一）。
func TestMixedStatsMatchesDays(t *testing.T) {
	h := newTestHandler(t)
	query := "start=2026-02-01&end=2026-02-03&dates=2026-02-17,2026-02-28"
	_, daysBody := doRequest(t, h, http.MethodGet, "/api/v1/days?"+query)
	code, statsBody := doRequest(t, h, http.MethodGet, "/api/v1/stats?"+query)
	if code != http.StatusOK {
		t.Fatalf("stats 混合模式状态码 = %d, want 200", code)
	}
	if !reflect.DeepEqual(daysBody["stats"], statsBody["stats"]) {
		t.Errorf("混合模式 stats 不一致: days 接口 %#v, stats 接口 %#v",
			daysBody["stats"], statsBody["stats"])
	}
	if daysBody["total_days"] != statsBody["total_days"] {
		t.Errorf("混合模式 total_days 不一致: %#v vs %#v",
			daysBody["total_days"], statsBody["total_days"])
	}
}
