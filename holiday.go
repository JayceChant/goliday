package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"
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
	layout    = "20060102"
	plaintext = "text/plain; charset=utf-8"
	jsontext  = "text/json; charset=utf-8"
)

var startYear = 2016

type YearCounter map[string]int
type Counters map[string]YearCounter
type TypeMap map[string]string

var HolidayType map[string]TypeMap = make(map[string]TypeMap)
var WeekendCount Counters = make(Counters)
var FestivalCount Counters = make(Counters)

func outputResponse(res *http.ResponseWriter, contentType string, a ...interface{}) {
	(*res).Header().Set("Content-Type", contentType)
	fmt.Fprintln(*res, a...)
}

func helpInfo(res http.ResponseWriter, req *http.Request) {
	outputResponse(&res, plaintext, HelpText)
}

func tomorrow(date *time.Time) time.Time {
	return date.AddDate(0, 0, 1)
}

func dateType(day time.Weekday) int {
	if time.Saturday == day || time.Sunday == day {
		return 1
	}
	return 0
}

func parseDateStr(date string) (time.Time, error) {
	return time.Parse(layout, date)
}

func getDateStr(date *time.Time) string {
	y, m, d := date.Date()
	return fmt.Sprintf("%04d%02d%02d", y, m, d)
}

func daysBetween(st, ed string) (int, bool) {
	dst, err1 := parseDateStr(st)
	ded, err2 := parseDateStr(ed)
	if nil != err1 || nil != err2 {
		log.Printf("time parse errors: %s:%s, %s:%s\n", st, err1, ed, err2)
		return 0, false
	}
	hours := ded.Sub(dst).Hours()
	log.Println(dst, ded, hours)
	return int(hours / 24), true
}

func loadFestival() YearCounter {
	var festivalAmend YearCounter
	buf, err := ioutil.ReadFile("festival.json")
	if nil != err {
		log.Println("ReadFile:", err)
		return nil
	}

	err = json.Unmarshal(buf, &festivalAmend)
	if nil != err {
		log.Println("Unmarshal:", err)
	}
	return festivalAmend
}

func loadYear(year int, amend YearCounter) {
	ht := make(map[string]string)
	wc := make(map[string]int)
	fc := make(map[string]int)
	date := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	// 获取明年 1月0日，亦即今年最后一天在今年的总日序，绕过闰年判断
	yearDays := time.Date(year+1, time.January, 0, 0, 0, 0, 0, time.UTC).YearDay()
	wkCnt, ftCnt := 0, 0
	for i := 1; i <= yearDays; i++ {
		dateStr := getDateStr(&date)
		// 为了实现查询区间左闭，计数器不包含当天，记录元旦到前一天的累计
		wc[dateStr] = wkCnt
		fc[dateStr] = ftCnt
		var flag int
		ok := false
		if nil != amend {
			flag, ok = amend[dateStr]
		}
		if !ok {
			flag = dateType(date.Weekday())
		}

		if 1 == flag {
			wkCnt++
		} else if 2 == flag {
			ftCnt++
		}
		ht[dateStr] = strconv.Itoa(flag)
		date = tomorrow(&date)
	}
	yearStr := strconv.Itoa(year)
	// 因为记录不包含当天，全年的计数以年位可以储存
	wc[yearStr] = wkCnt
	fc[yearStr] = ftCnt
	HolidayType[yearStr] = ht
	WeekendCount[yearStr] = wc
	FestivalCount[yearStr] = fc
}

func loadYears(stYear int) {
	amend := loadFestival()
	for y := stYear; y <= time.Now().Year(); y++ {
		loadYear(y, amend)
	}
}

func toJsonStr(o interface{}) string {
	buf, err := json.Marshal(o)
	if nil != err {
		fmt.Println("Marshal error:", err, o)
	}
	return fmt.Sprintf("%s", buf)
}

func holiday(res http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	y, ok := req.Form["y"]
	if ok {
		// y 参数只能传递一个，多余的忽略
		yType, ok := HolidayType[y[0]]
		if ok {
			outputResponse(&res, jsontext, toJsonStr(yType))
			return
		} else {
			outputResponse(&res, plaintext, "当前配置没有包含该年份！")
			helpInfo(res, req)
			return
		}
	}

	m, ok := req.Form["m"]
	if ok {
		// m 参数只能传递一个，多余的忽略
		st, err := parseDateStr(m[0] + "01")
		if nil != err {
			log.Println("parse month param error:", err)
			helpInfo(res, req)
			return
		}
		yType, ok := HolidayType[m[0][:4]]
		if ok {
			daysMap := make(TypeMap)
			date := st
			for i := 1; i <= 31; i++ {
				dStr := getDateStr(&date)
				dType, ok := yType[dStr]
				if ok {
					daysMap[dStr] = dType
				}
				date = tomorrow(&date)
				if date.Month() != st.Month() {
					break
				}
			}
			outputResponse(&res, jsontext, toJsonStr(daysMap))
			return
		}
	}

	d, ok := req.Form["d"]
	if ok {
		daysMap := make(TypeMap)
		for _, date := range d {
			yType, ok := HolidayType[date[:4]]
			if ok {
				dType, ok := yType[date]
				if ok {
					daysMap[date] = dType
				}
			}
		}
		outputResponse(&res, jsontext, toJsonStr(daysMap))
		return
	}
}

func dayTypeCountYear(counters Counters, year, st, ed string) (int, bool) {
	yCnt, ok := counters[year]
	if ok {
		hcst, ok1 := yCnt[st]
		hced, ok2 := yCnt[ed]
		if ok1 && ok2 {
			return hced - hcst, true
		}
	}
	return 0, false
}

func dayTypeCount(counters Counters, stDate, edDate string) (int, bool) {
	stYear, err1 := strconv.Atoi(stDate[:4])
	edYear, err2 := strconv.Atoi(edDate[:4])
	if nil != err1 || nil != err2 {
		log.Printf("year parse errors: %s:%s, %s:%s\n", stDate, err1, edDate, err2)
		return 0, false
	}

	var st, ed string
	result := 0
	for year := stYear; year <= edYear; year++ {
		yearStr := strconv.Itoa(year)
		if year != stYear {
			st = yearStr + "0101"
		} else {
			st = stDate
		}
		if year != edYear {
			ed = yearStr
		} else {
			ed = edDate
		}

		cnt, ok := dayTypeCountYear(counters, yearStr, st, ed)

		if !ok {
			return 0, false
		}
		result += cnt
	}
	return result, true
}

func getStartEnd(req *http.Request) (string, string, bool) {
	req.ParseForm()
	st, ok1 := req.Form["st"]
	ed, ok2 := req.Form["ed"]
	return st[0], ed[0], ok1 && ok2
}

func dayCountCommon(counters Counters, res http.ResponseWriter, req *http.Request) {
	st, ed, ok := getStartEnd(req)
	if ok {
		cnt, ok := dayTypeCount(counters, st, ed)
		if ok {
			fmt.Fprintln(res, cnt)
			return
		}
	}
	outputResponse(&res, plaintext, "查询不在当前配置范围内！")
	helpInfo(res, req)
}

func holidayCount(res http.ResponseWriter, req *http.Request) {
	st, ed, ok := getStartEnd(req)
	if ok {
		wc, ok1 := dayTypeCount(WeekendCount, st, ed)
		fc, ok2 := dayTypeCount(FestivalCount, st, ed)
		if ok1 && ok2 {
			if ok {
				outputResponse(&res, plaintext, wc+fc)
				return
			}
		}
	}
	outputResponse(&res, plaintext, "查询不在当前配置范围内！")
	helpInfo(res, req)
}

func weekendCount(res http.ResponseWriter, req *http.Request) {
	dayCountCommon(WeekendCount, res, req)
}

func festivalCount(res http.ResponseWriter, req *http.Request) {
	dayCountCommon(FestivalCount, res, req)
}

func workdayCount(res http.ResponseWriter, req *http.Request) {
	st, ed, ok := getStartEnd(req)
	if ok {
		wc, ok1 := dayTypeCount(WeekendCount, st, ed)
		fc, ok2 := dayTypeCount(FestivalCount, st, ed)
		if ok1 && ok2 {
			days, ok := daysBetween(st, ed)
			if ok {
				outputResponse(&res, plaintext, days-wc-fc)
				return
			}
		}
	}
	outputResponse(&res, plaintext, "查询不在当前配置范围内！")
	helpInfo(res, req)
}

func initWeb() {
	http.HandleFunc("/", helpInfo)
	http.HandleFunc("/holiday", holiday)
	http.HandleFunc("/holidayCount", holidayCount)
	http.HandleFunc("/weekendCount", weekendCount)
	http.HandleFunc("/festivalCount", festivalCount)
	http.HandleFunc("/workdayCount", workdayCount)
	err := http.ListenAndServe(":9091", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func main() {
	loadYears(startYear)
	//fmt.Println(dayTypeCount(WeekendCount, "20160101", "20181231"))
	initWeb()
	//test()
}

func test() {
	j, _ := json.Marshal(HolidayType)
	fmt.Printf("%s\n", j)
	j, _ = json.Marshal(WeekendCount)
	fmt.Printf("%s\n", j)
	j, _ = json.Marshal(FestivalCount)
	fmt.Printf("%s\n", j)
}
