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

	"github.com/JayceChant/goliday"
)

// dateLayout HTTP 层统一使用的 YYYY-MM-DD 日期格式。
const dateLayout = "2006-01-02"

// maxRangeDays days 明细接口允许的最大区间跨度（end - start 的天数）；
// stats 接口基于前缀和统计，不受此限制。
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
	// errInternalQuery 查询覆盖年份已在入口统一校验，核心包再报错仅作兜底。
	errInternalQuery = &apiError{http.StatusInternalServerError, "internal_error", "服务器内部错误"}
)

// internalQueryErr 查询意外失败的兜底：记录底层错误（供排查，调用方仅见
// 统一 500 文案）后返回 errInternalQuery。HTTP 与 gRPC 共用。
func internalQueryErr(op string, err error) *apiError {
	log.Printf("goliday-server: 查询意外失败（%s）: %v", op, err)
	return errInternalQuery
}

// yearNotLoadedError 构造未加载年份错误：message 列出升序去重的全部
// 未加载年份，HTTP 400 ↔ gRPC InvalidArgument 同源。
func yearNotLoadedError(years []int) *apiError {
	names := make([]string, len(years))
	for i, y := range years {
		names[i] = strconv.Itoa(y)
	}
	return &apiError{
		http.StatusBadRequest, "year_not_loaded",
		"查询范围包含未加载的年份: " + strings.Join(names, ", "),
	}
}

// checkYears 校验查询覆盖的年份均已加载（HTTP 与 gRPC 共用）；
// 任一未加载返回 year_not_loaded 错误。
func checkYears(cal *goliday.Calendar, years []int) *apiError {
	var missing []int
	for _, y := range years {
		if !cal.HasYear(y) {
			missing = append(missing, y)
		}
	}
	if missing == nil {
		return nil
	}
	return yearNotLoadedError(missing)
}

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

// handleHealthz 返回服务健康状态：状态、运行版本（构建期注入，见
// main.go version）、已加载年份与配置加载完成时间（运维确认「新配置
// 已生效」的依据）。
func (s *server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, struct {
		Status   string `json:"status"`
		Version  string `json:"version"`
		Years    []int  `json:"years"`
		LoadedAt string `json:"loaded_at"`
	}{
		Status:   "ok",
		Version:  version,
		Years:    s.store.Years(),
		LoadedAt: s.store.LoadedAt().UTC().Format(time.RFC3339),
	})
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

// fillStats 将统计结果填充为响应的 stats 字段：粗粒度恒为 workday/holiday
// 两键；细粒度为固定 5 键 MECE 计数（即使为 0 也输出，之和恒等于 total_days）。
// Fine 键为五个合法终态组合值（1/2/6/10/17）。
func fillStats(resp *daysResponse, st goliday.StatsResult, detailed bool) {
	if detailed {
		resp.Stats = map[string]int{
			"ordinary":      st.Fine[goliday.DayTypeWork],
			"weekend":       st.Fine[goliday.DayTypeRest],
			"festival":      st.Fine[goliday.DayTypeFestivalRest],
			"adjusted_rest": st.Fine[goliday.DayTypeAdjustedRestDay],
			"adjusted_work": st.Fine[goliday.DayTypeAdjustedWorkDay],
		}
	} else {
		resp.Stats = map[string]int{
			"workday": st.Coarse[goliday.DayTypeWork],
			"holiday": st.Coarse[goliday.DayTypeRest],
		}
	}
}

// buildQueryResponse 解析查询参数并构造响应体。参数规则：
//
//   - date：单日查询；detailed=false（默认）输出粗粒度 type/type_label，
//     true 输出细粒度数值与 DayType.String()；
//   - start+end：左闭右开区间，须成对出现、end>=start；days 明细接口
//     跨度不超过 maxRangeDays（stats 接口不限跨度）；
//   - dates：逗号分隔的日期列表，去重后升序；
//   - start+end 与 dates 并存时取并集（mode=list）；
//   - date 与其他参数并存时 date 优先，其余被忽略。
//
// 查询（单日/区间/离散/混合）覆盖的每个年份都必须已加载，否则返回
// year_not_loaded（400）并指明未加载年份，不再回退系统周休判断；
// 空区间（start==end）无覆盖年份，不校验并返回全零统计。
//
// 统计统一走前缀和路径（StatsRange/Stats），days 与 stats 接口同输入
// 统计完全一致；多日明细中的 type 始终为细粒度数值，detailed 仅切换
// stats 统计口径。
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

	if dateStr := q.Get("date"); dateStr != "" {
		// date 优先：与 start+end/dates 并存时忽略其他参数，按单日模式处理。
		d, aerr := parseDate(dateStr)
		if aerr != nil {
			return nil, aerr
		}
		if aerr := checkYears(s.calendar, []int{d.Year()}); aerr != nil {
			return nil, aerr
		}
		t, err := s.calendar.Query(d)
		if err != nil {
			return nil, internalQueryErr("单日 Query", err)
		}
		resp.Date = d.Format(dateLayout)
		shown := t
		if !detailed {
			shown = t.Coarse()
		}
		resp.Type = int(shown)
		resp.TypeLabel = shown.String()

		st, err := s.calendar.Stats([]time.Time{d}, detailed)
		if err != nil {
			return nil, internalQueryErr("单日 Stats", err)
		}
		fillStats(resp, st, detailed)
		resp.TotalDays = st.Total
		return resp, nil
	}

	// 仅 days 明细接口限制区间跨度；stats 基于前缀和不限跨度。
	maxSpan := 0
	if wantDays {
		maxSpan = maxRangeDays
	}
	mq, aerr := resolveMultiQuery(q.Get("start"), q.Get("end"), splitDatesParam(q.Get("dates")), maxSpan)
	if aerr != nil {
		return nil, aerr
	}
	if aerr := checkYears(s.calendar, mq.coveredYears()); aerr != nil {
		return nil, aerr
	}
	resp.Mode = mq.mode
	resp.Start = mq.start
	resp.End = mq.end

	if wantDays {
		dates := mq.datesForDays()
		resp.Days = make([]dayEntry, len(dates))
		for i, d := range dates {
			t, err := s.calendar.Query(d)
			if err != nil {
				return nil, internalQueryErr("多日明细 Query", err)
			}
			resp.Days[i] = dayEntry{Date: d.Format(dateLayout), Type: int(t), TypeLabel: t.String()}
		}
	}

	st, aerr := statsFor(s.calendar, mq, detailed)
	if aerr != nil {
		return nil, aerr
	}
	fillStats(resp, st, detailed)
	resp.TotalDays = st.Total
	return resp, nil
}

// multiQuery 多日查询（区间/离散/混合）的归一化结果。
//
// 区间不展开为逐日切片（大区间统计走前缀和）：仅保留 startT/endT 与
// 去重升序的 listDates，由调用方按需展开（days 明细）或差分统计（stats）。
// mode 区分仅区间（range）与离散/混合（list）；start/end 仅在 range
// 模式回显，与 HTTP 响应的省略行为一致。
type multiQuery struct {
	mode      string
	start     string
	end       string
	hasRange  bool
	startT    time.Time
	endT      time.Time
	listDates []time.Time
}

// coveredYears 返回该查询覆盖的全部年份（升序去重）：区间 [startT, endT)
// 覆盖的整年（含中间整年；空区间为空集）并上列表各日期年份。
// 区间部分复用根包 goliday.CoveredYears（与库内校验同一实现）。
func (mq *multiQuery) coveredYears() []int {
	years := goliday.CoveredYears(mq.startT, mq.endT)
	if !mq.hasRange {
		years = nil
	}
	for _, d := range mq.listDates {
		years = append(years, d.Year())
	}
	slices.Sort(years)
	return slices.Compact(years)
}

// datesForDays 展开该查询的完整日期集合（去重升序），供 days 明细使用；
// 仅在区间跨度已受限（≤366 天）的前提下调用。
func (mq *multiQuery) datesForDays() []time.Time {
	if !mq.hasRange {
		return mq.listDates
	}
	if len(mq.listDates) == 0 {
		return eachDay(mq.startT, mq.endT)
	}
	return mergeUnique(eachDay(mq.startT, mq.endT), mq.listDates)
}

// resolveMultiQuery 校验并归一化多日查询入参（HTTP 与 gRPC 共用）：
//
//   - start/end 须成对出现、均为合法日期、end>=start；maxSpanDays>0 时
//     跨度不超过 maxSpanDays（0 表示不限跨度）；
//   - dateStrs 为离散日期字符串列表（HTTP 逗号分列后、gRPC repeated 字段
//     原样传入），逐项解析后去重升序，任一非法即报错；
//   - 区间与列表并存时取并集（mode=list），仅区间 mode=range；
//   - 全部缺省返回 missing_query；
//   - 查询覆盖年份的加载校验由调用方经 coveredYears + checkYears 统一执行。
func resolveMultiQuery(startStr, endStr string, dateStrs []string, maxSpanDays int) (*multiQuery, *apiError) {
	if startStr == "" && endStr == "" && len(dateStrs) == 0 {
		return nil, errMissingQuery
	}

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
		if maxSpanDays > 0 && int(end.Sub(start).Hours()/24) > maxSpanDays {
			return nil, errRangeTooWide
		}
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

	mq := &multiQuery{hasRange: hasRange, startT: start, endT: end, listDates: listDays}
	switch {
	case hasRange && listDays != nil:
		// start+end 与 dates 并存：取并集去重升序。
		mq.mode = "list"
	case hasRange:
		mq.mode = "range"
		mq.start = start.Format(dateLayout)
		mq.end = end.Format(dateLayout)
	default:
		mq.mode = "list"
	}
	return mq, nil
}

// statsFor 计算多日查询的统计（HTTP 与 gRPC 共用），统一走前缀和路径：
// 区间用 StatsRange 差分；离散列表中剔除落在区间内的日期后单独分段
// 统计，再与区间统计相加（混合并集），因此 days 与 stats 接口、任意
// 跨度口径一致。覆盖年份已在入口校验，此处错误仅作兜底。
func statsFor(cal *goliday.Calendar, mq *multiQuery, detailed bool) (goliday.StatsResult, *apiError) {
	var st goliday.StatsResult
	if mq.hasRange {
		r, err := cal.StatsRange(mq.startT, mq.endT, detailed)
		if err != nil {
			return goliday.StatsResult{}, internalQueryErr("区间 StatsRange", err)
		}
		st = r
	}
	if len(mq.listDates) > 0 {
		dates := mq.listDates
		if mq.hasRange {
			dates = filterOutsideRange(dates, mq.startT, mq.endT)
		}
		if len(dates) > 0 {
			r, err := cal.Stats(dates, detailed)
			if err != nil {
				return goliday.StatsResult{}, internalQueryErr("列表 Stats", err)
			}
			st = addStats(st, r)
		}
	}
	return st, nil
}

// addStats 将两组统计相加（Total 与 Coarse/Fine 各计数键）。
func addStats(a, b goliday.StatsResult) goliday.StatsResult {
	a.Total += b.Total
	if a.Coarse == nil {
		a.Coarse = make(map[goliday.DayType]int, len(b.Coarse))
	}
	for k, v := range b.Coarse {
		a.Coarse[k] += v
	}
	if b.Fine != nil {
		if a.Fine == nil {
			a.Fine = make(map[goliday.DayType]int, len(b.Fine))
		}
		for k, v := range b.Fine {
			a.Fine[k] += v
		}
	}
	return a
}

// filterOutsideRange 返回列表中不落在左闭右开区间 [start, end) 内的日期。
func filterOutsideRange(dates []time.Time, start, end time.Time) []time.Time {
	out := make([]time.Time, 0, len(dates))
	for _, d := range dates {
		if d.Before(start) || !d.Before(end) {
			out = append(out, d)
		}
	}
	return out
}

// ---- 参数解析 helper ----

// parseDate 严格解析 YYYY-MM-DD，失败映射为 invalid_date API 错误；
// 解析逻辑复用根包导出的 goliday.ParseDate（与服务层配置加载同口径）。
func parseDate(s string) (time.Time, *apiError) {
	t, err := goliday.ParseDate(s)
	if err != nil {
		return time.Time{}, &apiError{
			http.StatusBadRequest, "invalid_date", err.Error(),
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
