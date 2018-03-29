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

const layout = "20060102"

type YearCounter map[string]int
type Counters map[string]YearCounter

var FestivalAmend YearCounter
var HolidayType map[string]interface{} = make(map[string]interface{})
var WeekendCount Counters = make(Counters)
var FestivalCount Counters = make(Counters)
var HolidayCount Counters = make(Counters)

func helpInfo(res http.ResponseWriter, req *http.Request) {
	fmt.Fprintln(res, HelpText)
}

func tomorrow(date *time.Time) time.Time {
	return date.AddDate(0, 0, 1)
}

func dateType(dateStr string, day time.Weekday) int {
	if nil != FestivalAmend {
		flag, ok := FestivalAmend[dateStr]
		if ok {
			return flag
		}
	}
	if time.Saturday == day || time.Sunday == day {
		return 1
	}
	return 0
}

func parseDateStr(date string) (time.Time, error) {
	return time.Parse(layout, date)
}

func daysBetween(st, ed string) (int, bool) {
	dst, err1 := parseDateStr(st)
	ded, err2 := parseDateStr(ed)
	if nil != err1 || nil != err2 {
		log.Println("time parse errors:", err1, ", ", err2)
		return 0, false
	}
	hours := ded.Sub(dst).Hours()
	log.Println(dst, ded, hours)
	return int(hours / 24), true
}

func loadFestival() {
	buf, err := ioutil.ReadFile("festival.json")
	if nil != err {
		log.Println("ReadFile:", err)
		return
	}

	err = json.Unmarshal(buf, &FestivalAmend)
	if nil != err {
		log.Println("Unmarshal:", err)
	}
}

func getDateStr(date *time.Time) string {
	y, m, d := date.Date()
	return fmt.Sprintf("%04d%02d%02d", y, m, d)
}

func enumYear(year int) {
	ht := make(map[string]string)
	wc := make(map[string]int)
	fc := make(map[string]int)
	hc := make(map[string]int)
	date := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	yearDays := time.Date(year+1, time.January, 0, 0, 0, 0, 0, time.UTC).YearDay()
	wkCnt, ftCnt := 0, 0
	for i := 1; i <= yearDays; i++ {
		dateStr := getDateStr(&date)
		// 为了实现查询区间左闭，计数器不包含当天，记录元旦到前一天的累计
		wc[dateStr] = wkCnt
		fc[dateStr] = ftCnt
		hc[dateStr] = wkCnt + ftCnt
		flag := dateType(dateStr, date.Weekday())
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
	hc[yearStr] = wkCnt + ftCnt
	HolidayType[yearStr] = ht
	WeekendCount[yearStr] = wc
	FestivalCount[yearStr] = fc
	HolidayCount[yearStr] = hc
}

func enumYears(stYear int) {
	for y := stYear; y <= time.Now().Year(); y++ {
		enumYear(y)
	}
}

func holiday(res http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	res.Header().Set("Content-Type", "text/json; charset=utf-8")
	y, ok := req.Form["y"]
	if ok {
		// y 参数只能传递一个，多余的忽略
		year, ok := HolidayType[y[0]]
		if ok {
			j, _ := json.Marshal(year)
			fmt.Fprintf(res, "%s\n", j)
			return
		} else {
			fmt.Fprintln(res, "当前配置没有包含该年份！")
		}
	}

	//	m, ok := req.Form["m"]
	//	if ok {
	//		// m 参数只能传递一个，多余的忽略
	//		root[m[0]] = enumYear(m[0])
	//		fmt.Fprintln(res, root)
	//		return
	//	}
	helpInfo(res, req)
}

func dayCount(counters Counters, st, ed string) (int, bool) {
	year, ok := counters[st[:4]]
	if ok {
		hcst, ok1 := year[st]
		hced, ok2 := year[ed]
		if ok1 && ok2 {
			return hced - hcst, true
		}
	}
	return 0, false
}

func getStartEnd(req *http.Request) (string, string, bool) {
	st, ok1 := req.Form["st"]
	ed, ok2 := req.Form["ed"]
	return st[0], ed[0], ok1 && ok2
}

func dayCountCommon(counters Counters, res http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	res.Header().Set("Content-Type", "text/text; charset=utf-8")
	st, ed, ok := getStartEnd(req)
	if ok {
		cnt, ok := dayCount(counters, st, ed)
		if ok {
			fmt.Fprintln(res, cnt)
			return
		}
	}
	fmt.Fprintln(res, "查询不在当前配置范围内！")
	helpInfo(res, req)
}

func holidayCount(res http.ResponseWriter, req *http.Request) {
	dayCountCommon(HolidayCount, res, req)
}

func weekendCount(res http.ResponseWriter, req *http.Request) {
	dayCountCommon(WeekendCount, res, req)
}

func festivalCount(res http.ResponseWriter, req *http.Request) {
	dayCountCommon(FestivalCount, res, req)
}

func workdayCount(res http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	res.Header().Set("Content-Type", "text/text; charset=utf-8")
	st, ed, ok := getStartEnd(req)
	if ok {
		cnt, ok := dayCount(HolidayCount, st, ed)
		if ok {
			days, ok := daysBetween(st, ed)
			if ok {
				fmt.Fprintln(res, days-cnt)
				return
			}
		}
	}
	fmt.Fprintln(res, "查询不在当前配置范围内！")
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
	loadFestival()
	enumYears(2018)
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
	j, _ = json.Marshal(HolidayCount)
	fmt.Printf("%s\n", j)
}
