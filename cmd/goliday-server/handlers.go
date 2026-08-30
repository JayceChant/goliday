package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"goliday"
)

// dateLayout HTTP 层统一使用的 YYYY-MM-DD 日期格式。
const dateLayout = "2006-01-02"

// maxRangeDays 区间查询允许的最大跨度（end - start 的天数）。
const maxRangeDays = 366

// server 持有只读的年份存储与日历索引，构造完成后可被多个 goroutine 并发调用。
type server struct {
	store    *goliday.Store
	calendar *goliday.Calendar
}

// newServer 构造 server。
func newServer(store *goliday.Store, calendar *goliday.Calendar) *server {
	return &server{store: store, calendar: calendar}
}

// newHandler 构造完整处理链：访问日志 → panic 恢复 → 路由分发。
func newHandler(store *goliday.Store, calendar *goliday.Calendar) http.Handler {
	return loggingMiddleware(recoverMiddleware(newRouter(newServer(store, calendar))))
}

// newRouter 注册全部路由：带方法的精确模式优先匹配；同路径的无方法模式接管
// 方法不符的请求（405），兜底 "/" 接管未匹配路径（404），统一输出 JSON 错误。
func newRouter(s *server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/days", s.handleDays)
	mux.HandleFunc("GET /api/v1/stats", s.handleStats)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("/api/v1/days", methodNotAllowed)
	mux.HandleFunc("/api/v1/stats", methodNotAllowed)
	mux.HandleFunc("/healthz", methodNotAllowed)
	mux.HandleFunc("/", notFound)
	return mux
}

// ---- 中间件 ----

// statusRecorder 包装 ResponseWriter 以记录写出的状态码，供日志中间件读取。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// loggingMiddleware 记录每个请求的方法、路径、状态码与耗时。
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		log.Printf("%s %s %d %s", r.Method, r.URL.RequestURI(), rec.status, time.Since(start))
	})
}

// recoverMiddleware 捕获下游 panic，统一输出 500 错误 JSON，避免进程退出。
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("处理 %s %s 发生 panic: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
				writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ---- 响应 helper ----

// apiError 统一 API 错误：status 为 HTTP 状态码，code 为机器可读错误码。
type apiError struct {
	status  int
	code    string
	message string
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

// HTTP 与 gRPC 共用的参数错误，code 与 message 文案即两个协议的错误契约
// （gRPC 侧映射为 codes.InvalidArgument + 相同 message）。
var (
	errInvalidDetailed = &apiError{http.StatusBadRequest, "invalid_detailed", "参数 detailed 须为布尔值"}
	errMissingQuery    = &apiError{http.StatusBadRequest, "missing_query", "缺少查询参数：date、start+end 或 dates"}
	errRangeUnpaired   = &apiError{http.StatusBadRequest, "invalid_range", "start 与 end 须成对出现"}
	errRangeOrder      = &apiError{http.StatusBadRequest, "invalid_range", "end 不得早于 start"}
	errRangeTooWide    = &apiError{http.StatusBadRequest, "invalid_range", fmt.Sprintf("区间跨度超过 %d 天", maxRangeDays)}
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("写响应失败: %v", err)
	}
}

// writeError 输出统一错误结构 {"error":{"code":"...","message":"..."}}。
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, errorEnvelope{Error: errorBody{Code: code, Message: msg}})
}

func notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "未找到资源: "+r.URL.Path)
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "GET")
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "方法不被允许: "+r.Method)
}

// ---- 路由处理器 ----

// handleHealthz 返回服务健康状态与已加载年份。
func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, struct {
		Status string `json:"status"`
		Years  []int  `json:"years"`
	}{Status: "ok", Years: s.store.Years()})
}

// handleDays 返回日期类型明细与统计。
func (s *server) handleDays(w http.ResponseWriter, r *http.Request) {
	s.serveQuery(w, r, true)
}

// handleStats 与 days 完全相同的入参与统计口径，但不返回 days 明细。
func (s *server) handleStats(w http.ResponseWriter, r *http.Request) {
	s.serveQuery(w, r, false)
}

// serveQuery days 与 stats 共用的入口：wantDays 控制是否填充 days 明细。
func (s *server) serveQuery(w http.ResponseWriter, r *http.Request, wantDays bool) {
	resp, aerr := s.buildQueryResponse(r.URL.Query(), wantDays)
	if aerr != nil {
		writeError(w, aerr.status, aerr.code, aerr.message)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// dayEntry 多日明细中的单日条目。
type dayEntry struct {
	Date      string `json:"date"`
	Type      int    `json:"type"`
	TypeLabel string `json:"type_label"`
}

// daysResponse days 与 stats 接口的共用响应体：单日模式填充顶层
// Date/Type/TypeLabel 而不输出 days 数组；stats 接口不填充 Days，
// 由 omitempty 省略 days 字段。
type daysResponse struct {
	Mode      string         `json:"mode,omitempty"`
	Date      string         `json:"date,omitempty"`
	Type      int            `json:"type,omitempty"`
	TypeLabel string         `json:"type_label,omitempty"`
	Start     string         `json:"start,omitempty"`
	End       string         `json:"end,omitempty"`
	TotalDays int            `json:"total_days"`
	Days      []dayEntry     `json:"days,omitempty"`
	Stats     map[string]int `json:"stats"`
}

// buildQueryResponse 解析查询参数并构造响应体。参数规则：
//
//   - date：单日查询；detailed=false（默认）输出粗粒度 type/type_label，
//     true 输出细粒度数值与 DayType.String()；
//   - start+end：左闭右开区间，须成对出现、end>=start 且跨度不超过 maxRangeDays；
//   - dates：逗号分隔的日期列表，去重后升序；
//   - start+end 与 dates 并存时取并集（mode=list）；
//   - date 与其他参数并存时 date 优先，其余被忽略。
//
// 多日明细中的 type 始终为细粒度数值，detailed 仅切换 stats 统计口径。
func (s *server) buildQueryResponse(q url.Values, wantDays bool) (*daysResponse, *apiError) {
	detailed := false
	if v := q.Get("detailed"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, errInvalidDetailed
		}
		detailed = b
	}

	resp := &daysResponse{}
	var dates []time.Time

	if dateStr := q.Get("date"); dateStr != "" {
		// date 优先：与 start+end/dates 并存时忽略其他参数，按单日模式处理。
		d, aerr := parseDate(dateStr)
		if aerr != nil {
			return nil, aerr
		}
		dates = []time.Time{d}
		resp.Date = d.Format(dateLayout)
		t := s.calendar.Query(d)
		shown := t
		if !detailed {
			shown = t.Coarse()
		}
		resp.Type = int(shown)
		resp.TypeLabel = shown.String()
	} else {
		mq, aerr := resolveMultiQuery(q.Get("start"), q.Get("end"), splitDatesParam(q.Get("dates")))
		if aerr != nil {
			return nil, aerr
		}
		resp.Mode = mq.mode
		resp.Start = mq.start
		resp.End = mq.end
		dates = mq.dates

		if wantDays {
			resp.Days = make([]dayEntry, len(dates))
			for i, d := range dates {
				t := s.calendar.Query(d)
				resp.Days[i] = dayEntry{Date: d.Format(dateLayout), Type: int(t), TypeLabel: t.String()}
			}
		}
	}

	st := s.calendar.Stats(dates, detailed)
	if detailed {
		// 固定 5 个细粒度键，组合日交叉计数，即使为 0 也输出。
		resp.Stats = map[string]int{
			"ordinary":   st.Fine[goliday.DayTypeOrdinary],
			"compensate": st.Fine[goliday.DayTypeCompensate],
			"weekend":    st.Fine[goliday.DayTypeWeekend],
			"festival":   st.Fine[goliday.DayTypeFestival],
			"adjusted":   st.Fine[goliday.DayTypeAdjusted],
		}
	} else {
		resp.Stats = map[string]int{
			"workday": st.Coarse[goliday.DayTypeWorkday],
			"holiday": st.Coarse[goliday.DayTypeHoliday],
		}
	}
	resp.TotalDays = st.Total
	return resp, nil
}

// multiQuery 多日查询（区间/离散/混合）的归一化结果：dates 为去重升序的
// 日期集合；mode 区分仅区间（range）与离散/混合（list）；start/end 仅在
// range 模式回显，与 HTTP 响应的省略行为一致。
type multiQuery struct {
	mode  string
	start string
	end   string
	dates []time.Time
}

// resolveMultiQuery 校验并归一化多日查询入参（HTTP 与 gRPC 共用）：
//
//   - start/end 须成对出现、均为合法日期、end>=start 且跨度不超过 maxRangeDays；
//   - dateStrs 为离散日期字符串列表（HTTP 逗号分列后、gRPC repeated 字段原样
//     传入），逐项解析后去重升序，任一非法即报错；
//   - 区间与列表并存时取并集（mode=list），仅区间 mode=range；
//   - 全部缺省返回 missing_query。
func resolveMultiQuery(startStr, endStr string, dateStrs []string) (*multiQuery, *apiError) {
	if startStr == "" && endStr == "" && len(dateStrs) == 0 {
		return nil, errMissingQuery
	}

	var rangeDays []time.Time
	var start, end time.Time
	hasRange := false
	if startStr != "" || endStr != "" {
		if startStr == "" || endStr == "" {
			return nil, errRangeUnpaired
		}
		var aerr *apiError
		if start, aerr = parseDate(startStr); aerr != nil {
			return nil, aerr
		}
		if end, aerr = parseDate(endStr); aerr != nil {
			return nil, aerr
		}
		if end.Before(start) {
			return nil, errRangeOrder
		}
		if int(end.Sub(start).Hours()/24) > maxRangeDays {
			return nil, errRangeTooWide
		}
		rangeDays = eachDay(start, end)
		hasRange = true
	}

	var listDays []time.Time
	if len(dateStrs) > 0 {
		ds, aerr := dedupDateList(dateStrs)
		if aerr != nil {
			return nil, aerr
		}
		listDays = ds
	}

	mq := &multiQuery{}
	switch {
	case hasRange && listDays != nil:
		// start+end 与 dates 并存：取并集去重升序。
		mq.dates = mergeUnique(rangeDays, listDays)
		mq.mode = "list"
	case hasRange:
		mq.dates = rangeDays
		mq.mode = "range"
		mq.start = start.Format(dateLayout)
		mq.end = end.Format(dateLayout)
	default:
		mq.dates = listDays
		mq.mode = "list"
	}
	return mq, nil
}

// ---- 参数解析 helper ----

// parseDate 严格解析 YYYY-MM-DD。time.Parse 对不存在的日期（如 2026-02-30）
// 会进位而非报错，故须回格式化比对兜底。
func parseDate(s string) (time.Time, *apiError) {
	t, err := time.Parse(dateLayout, s)
	if err != nil || t.Format(dateLayout) != s {
		return time.Time{}, &apiError{
			http.StatusBadRequest, "invalid_date",
			fmt.Sprintf("非法日期 %q：须为 YYYY-MM-DD 格式的有效日期", s),
		}
	}
	return t, nil
}

// splitDatesParam 将 HTTP dates 参数按逗号分列；参数缺省（空串）时返回 nil，
// 表示未提供离散列表。
func splitDatesParam(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// dedupDateList 逐项解析日期字符串列表（HTTP 逗号分列结果或 gRPC repeated
// 字段原值），去重后升序返回。
func dedupDateList(parts []string) ([]time.Time, *apiError) {
	seen := make(map[time.Time]struct{}, len(parts))
	out := make([]time.Time, 0, len(parts))
	for _, p := range parts {
		d, aerr := parseDate(strings.TrimSpace(p))
		if aerr != nil {
			return nil, aerr
		}
		if _, ok := seen[d]; !ok {
			seen[d] = struct{}{}
			out = append(out, d)
		}
	}
	slices.SortFunc(out, func(a, b time.Time) int { return a.Compare(b) })
	return out, nil
}

// eachDay 展开左闭右开区间 [start, end) 内的每一天。
func eachDay(start, end time.Time) []time.Time {
	out := make([]time.Time, 0, int(end.Sub(start).Hours()/24))
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		out = append(out, d)
	}
	return out
}

// mergeUnique 合并多组日期并去重，升序返回。
func mergeUnique(groups ...[]time.Time) []time.Time {
	seen := make(map[time.Time]struct{})
	var out []time.Time
	for _, g := range groups {
		for _, d := range g {
			if _, ok := seen[d]; !ok {
				seen[d] = struct{}{}
				out = append(out, d)
			}
		}
	}
	slices.SortFunc(out, func(a, b time.Time) int { return a.Compare(b) })
	return out
}
