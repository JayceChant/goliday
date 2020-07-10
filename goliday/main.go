package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/JayceChant/goliday"
	"github.com/JayceChant/goliday/dateutil"
)

const HelpText = `Jayce Chant's Holiday Service

Usage:
1. /                帮助信息，也就是当前页面
2. /holiday         返回一段时间内的每天是否节假日，0-工作日/1-周末/2-节假日，参数支持 y=2018（一年）， m=201801（一个月）, d=20180101（一天）, 结果用 json 返回。d 允许传多个参数。
3. /holidayCount    返回一段时间内的节假日(1/2)数量，?st=20180101&ed=20180205，返回 [st, ed) 范围内的结果，st < ed
4. /weekendCount    同 3，仅统计周末(1)
5. /festivalCount   同 3，仅统计节日(2)
6. /workdayCount    同 3，仅统计工作日(0)，相当于总天数减掉 3 的结果
`

const (
	mimePlainText = "text/plain; charset=utf-8"
	mimeJsonText  = "text/json; charset=utf-8"
	startYear     = 2016
)

func outputResponse(res http.ResponseWriter, contentType string, a ...interface{}) {
	res.Header().Set("Content-Type", contentType)
	fmt.Fprintln(res, a...)
}

func helpInfoHandler(res http.ResponseWriter, req *http.Request) {
	outputResponse(res, mimePlainText, HelpText)
}

func toJSONStr(o interface{}) string {
	buf, err := json.Marshal(o)
	if nil != err {
		fmt.Println("Marshal error:", err, o)
	}
	return fmt.Sprintf("%s", buf)
}

func holidayHandler(res http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	y, ok := req.Form["y"]
	if ok {
		// y 参数只能传递一个，多余的忽略
		yType, ok := goliday.GetDayTypesByYear(y[0])
		if ok {
			outputResponse(res, mimeJsonText, toJSONStr(yType))
			return
		} else {
			outputResponse(res, mimePlainText, "当前配置没有包含该年份！")
			helpInfoHandler(res, req)
			return
		}
	}

	m, ok := req.Form["m"]
	if ok {
		// m 参数只能传递一个，多余的忽略
		mType, ok := goliday.GetDayTypesByMonth(m[0])
		if ok {
			outputResponse(res, mimeJsonText, toJSONStr(mType))
			return
		} else {
			outputResponse(res, mimePlainText, "当前配置没有包含该月份！")
			helpInfoHandler(res, req)
			return
		}
	}

	d, ok := req.Form["d"]
	if ok {
		dTypes, ok := goliday.GetDayTypesByDates(d)
		if ok {
			outputResponse(res, mimeJsonText, toJSONStr(dTypes))
			return
		}
	}
}

func getStartEnd(req *http.Request) (string, string, bool) {
	req.ParseForm()
	st, ok1 := req.Form["st"]
	ed, ok2 := req.Form["ed"]
	return st[0], ed[0], ok1 && ok2
}

func holidayCountHandler(res http.ResponseWriter, req *http.Request) {
	st, ed, ok := getStartEnd(req)
	if ok {
		wc, ok1 := goliday.DayCountCrossYear(dateutil.Weekend, st, ed)
		fc, ok2 := goliday.DayCountCrossYear(dateutil.Festival, st, ed)
		if ok1 && ok2 {
			if ok {
				outputResponse(res, mimePlainText, wc+fc)
				return
			}
		}
	}
	outputResponse(res, mimePlainText, "查询不在当前配置范围内！")
	helpInfoHandler(res, req)
}

func weekendCountHandler(res http.ResponseWriter, req *http.Request) {
	st, ed, ok := getStartEnd(req)
	if ok {
		cnt, ok := goliday.DayCountCrossYear(dateutil.Weekend, st, ed)
		if ok {
			fmt.Fprintln(res, cnt)
			return
		}
	}
	outputResponse(res, mimePlainText, "查询不在当前配置范围内！")
	helpInfoHandler(res, req)
}

func festivalCountHandler(res http.ResponseWriter, req *http.Request) {
	st, ed, ok := getStartEnd(req)
	if ok {
		cnt, ok := goliday.DayCountCrossYear(dateutil.Festival, st, ed)
		if ok {
			fmt.Fprintln(res, cnt)
			return
		}
	}
	outputResponse(res, mimePlainText, "查询不在当前配置范围内！")
	helpInfoHandler(res, req)
}

func workdayCountHandler(res http.ResponseWriter, req *http.Request) {
	st, ed, ok := getStartEnd(req)
	if ok {
		wc, ok1 := goliday.DayCountCrossYear(dateutil.Weekend, st, ed)
		fc, ok2 := goliday.DayCountCrossYear(dateutil.Festival, st, ed)
		if ok1 && ok2 {
			days, ok := dateutil.DaysBetween(st, ed)
			if ok {
				outputResponse(res, mimePlainText, days-wc-fc)
				return
			}
		}
	}
	outputResponse(res, mimePlainText, "查询不在当前配置范围内！")
	helpInfoHandler(res, req)
}

func initWeb() {
	http.HandleFunc("/", helpInfoHandler)
	http.HandleFunc("/holiday", holidayHandler)
	http.HandleFunc("/holidayCount", holidayCountHandler)
	http.HandleFunc("/weekendCount", weekendCountHandler)
	http.HandleFunc("/festivalCount", festivalCountHandler)
	http.HandleFunc("/workdayCount", workdayCountHandler)
	log.Println("========== map_route_handler finished. Ready to start server. ==========")
	err := http.ListenAndServe(":9091", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func main() {
	goliday.LoadYears(startYear)
	initWeb()
}
