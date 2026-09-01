// 白盒测试（package main）：经 newHandler 构造完整处理链，fuzz 任意
// 查询参数组合，验证 /api/v1/days 与 /api/v1/stats 的鲁棒性与响应不变量。
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/JayceChant/goliday"
)

// fuzzHandler 进程内仅构造一次处理链：store/calendar 构造完成后只读、
// 并发安全，fuzz workers 可安全共享（用 sync.OnceValue 惰性初始化）。
var fuzzHandler = sync.OnceValue(func() http.Handler {
	store, err := goliday.LoadDir("../../testdata")
	if err != nil {
		panic("fuzz 加载测试配置失败: " + err.Error())
	}
	return newHandler(store, goliday.NewCalendar(store))
})

// FuzzDaysHandler 不变量：任意参数组合不 panic；状态码仅 200/400（含
// year_not_loaded 与 days 跨度超限场景）；响应恒为合法 JSON；200 时
// 单日模式 total_days == 1，多日模式 days 升序唯一且 total_days ==
// len(days)；stats 接口不因跨度报错；粗粒度 stats 之和 == total_days，
// 细粒度（组合日交叉计数）之和 >= total_days。
func FuzzDaysHandler(f *testing.F) {
	f.Add("2026-02-20", "", "", "", "")
	f.Add("2026-02-17", "2026-02-01", "2026-02-28", "2026-03-08", "true")
	f.Add("", "2026-02-01", "2026-02-28", "", "false")
	f.Add("2026-02-30", "2026-03-05", "2026-03-01", "abc,2026-01-01", "yes")
	f.Add("", "", "", "2026-02-16,2026-02-28,2026-02-17,2026-02-16", "1")
	f.Add("", "2026-01-01", "2027-06-01", "", "")
	f.Add("2027-05-01", "", "", "", "")
	f.Add("", "2025-01-01", "2027-01-01", "", "")
	f.Add("", "2025-01-01", "2027-01-01", "2030-01-01", "")
	f.Fuzz(func(t *testing.T, dateParam, start, end, dates, detailed string) {
		h := fuzzHandler()
		for _, path := range []string{"/api/v1/days", "/api/v1/stats"} {
			q := url.Values{}
			if dateParam != "" {
				q.Set("date", dateParam)
			}
			if start != "" {
				q.Set("start", start)
			}
			if end != "" {
				q.Set("end", end)
			}
			if dates != "" {
				q.Set("dates", dates)
			}
			if detailed != "" {
				q.Set("detailed", detailed)
			}

			req := httptest.NewRequest(http.MethodGet, path+"?"+q.Encode(), nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req) // panic 由 fuzz 引擎报告

			if rec.Code != http.StatusOK && rec.Code != http.StatusBadRequest {
				t.Fatalf("%s 状态码 = %d，期望 200/400\n响应体: %s", path, rec.Code, rec.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("%s 响应不是合法 JSON: %v\n响应体: %s", path, err, rec.Body.String())
			}
			if rec.Code != http.StatusOK {
				continue
			}

			total, ok := body["total_days"].(float64)
			if !ok {
				t.Fatalf("%s 响应缺少数值型 total_days: %#v", path, body["total_days"])
			}
			// 单日模式：date 参数优先，仅统计 1 天。
			if _, single := body["date"]; single {
				if total != 1 {
					t.Fatalf("%s 单日模式 total_days = %v，期望 1", path, total)
				}
			} else if days, hasDays := body["days"].([]any); hasDays {
				// 多日模式：明细升序唯一且与 total_days 一致。
				if total != float64(len(days)) {
					t.Fatalf("%s total_days = %v，len(days) = %d", path, total, len(days))
				}
				prev := ""
				for i, d := range days {
					m, ok := d.(map[string]any)
					if !ok {
						t.Fatalf("%s days[%d] 不是 JSON 对象: %#v", path, i, d)
					}
					cur, _ := m["date"].(string)
					if i > 0 && cur <= prev {
						t.Fatalf("%s days 非升序或重复: %s 后出现 %s", path, prev, cur)
					}
					prev = cur
				}
			}

			st, ok := body["stats"].(map[string]any)
			if !ok {
				t.Fatalf("%s 响应缺少 stats 对象: %#v", path, body["stats"])
			}
			sum := 0
			for _, v := range st {
				n, _ := v.(float64)
				sum += int(n)
			}
			if _, fine := st["ordinary"]; fine {
				// 细粒度交叉计数：组合日对每个标志位各计 1，之和 >= 总天数。
				if sum < int(total) {
					t.Fatalf("%s 细粒度 stats 之和 %d < total_days %v（stats: %v）", path, sum, total, st)
				}
			} else {
				// 粗粒度：workday + holiday == 总天数。
				if sum != int(total) {
					t.Fatalf("%s 粗粒度 stats 之和 %d != total_days %v（stats: %v）", path, sum, total, st)
				}
			}
		}
	})
}
