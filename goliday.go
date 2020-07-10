package goliday

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"strconv"
	"time"

	"github.com/JayceChant/goliday/dateutil"
)

const (
	customDir      = "custom/"
	festivalConfig = customDir + "festival.json"
)

type (
	// DateToDayType ...
	DateToDayType  map[string]dateutil.DayType
	yearCounter    map[string]int
	countersByYear map[string]yearCounter
)

var (
	weekendCount   = make(countersByYear)
	festivalCount  = make(countersByYear)
	dayTypesByYear = make(map[string]DateToDayType)
)

// DayCountCrossYear ...
func DayCountCrossYear(dayType dateutil.DayType, stDate, edDate string) (int, bool) {
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

		// TODO: 判断不严谨，考虑要不要将所有类型都统计，避免做差
		var counters countersByYear
		if dayType == dateutil.Weekend {
			counters = weekendCount
		} else {
			counters = festivalCount
		}
		cnt, ok := dayTypeCount(counters, yearStr, st, ed)

		if !ok {
			return 0, false
		}
		result += cnt
	}
	return result, true
}

func dayTypeCount(counters countersByYear, year, st, ed string) (int, bool) {
	counter, ok := counters[year]
	if ok {
		tillSt, ok1 := counter[st]
		tillEd, ok2 := counter[ed]
		if ok1 && ok2 {
			return tillEd - tillSt, true
		}
	}
	return 0, false
}

// GetDayTypesByYear ...
func GetDayTypesByYear(year string) (ret DateToDayType, ok bool) {
	ret, ok = dayTypesByYear[year]
	return
}

// GetDayTypesByMonth ...
func GetDayTypesByMonth(month string) (DateToDayType, bool) {
	st, err := dateutil.ParseDateStr(month + "01")
	if nil != err {
		log.Println("parse month param error:", err)
		return nil, false
	}
	yType, ok := dayTypesByYear[month[:4]]
	if ok {
		dayTypes := make(DateToDayType)
		date := st
		for i := 1; i <= 31; i++ {
			dStr := dateutil.DateStr(&date)
			dType, ok := yType[dStr]
			if ok {
				dayTypes[dStr] = dType
			}
			date = dateutil.NextDay(&date)
			if date.Month() != st.Month() {
				break
			}
		}
		return dayTypes, true
	}
	return nil, false
}

// GetDayTypesByDates ...
func GetDayTypesByDates(dates []string) (DateToDayType, bool) {
	dayTypes := make(DateToDayType)
	for _, date := range dates {
		yType, ok := dayTypesByYear[date[:4]]
		if ok {
			dType, ok := yType[date]
			if ok {
				dayTypes[date] = dType
			}
		}
	}
	return dayTypes, true
}

// LoadYears ...
func LoadYears(stYear int) {
	amend := loadFestival()
	for y := stYear; y <= time.Now().Year(); y++ {
		loadYear(y, amend)
	}
	log.Println("========== load_data finished. ==========")
}

func loadYear(year int, amend DateToDayType) {
	dayTypes := make(DateToDayType)
	weekendCounts := make(yearCounter)
	festivalCounts := make(yearCounter)
	date := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	// 获取明年 1月0日，亦即今年最后一天在今年的总日序，绕过闰年判断
	yearDayNum := time.Date(year+1, time.January, 0, 0, 0, 0, 0, time.UTC).YearDay()
	weekendCounter, festivalCounter := 0, 0
	for i := 1; i <= yearDayNum; i++ {
		dateStr := dateutil.DateStr(&date)
		// 为了实现查询区间左闭，计数器不包含当天，记录元旦到前一天的累计
		weekendCounts[dateStr] = weekendCounter
		festivalCounts[dateStr] = festivalCounter
		var dateType dateutil.DayType
		ok := false
		if nil != amend {
			dateType, ok = amend[dateStr]
		}
		if !ok {
			dateType = dateutil.WeekdayToDayType(date.Weekday())
		}

		if dateutil.Weekend == dateType {
			weekendCounter++
		} else if dateutil.Festival == dateType {
			festivalCounter++
		}
		dayTypes[dateStr] = dateType
		date = dateutil.NextDay(&date)
	}
	yearStr := strconv.Itoa(year)
	// 因为记录不包含当天，全年的计数以年为键储存
	weekendCounts[yearStr] = weekendCounter
	festivalCounts[yearStr] = festivalCounter
	dayTypesByYear[yearStr] = dayTypes
	weekendCount[yearStr] = weekendCounts
	festivalCount[yearStr] = festivalCounts
}

// loadFestival ...
// TODO: 为了避免单个配置文件过长，改为按年份存放
func loadFestival() DateToDayType {
	var dayTypeAmend DateToDayType
	buf, err := ioutil.ReadFile(festivalConfig)
	if nil != err {
		log.Println("ReadFile:", err)
		return nil
	}
	fromJSONStr(buf, &dayTypeAmend)
	return dayTypeAmend
}

func fromJSONStr(data []byte, vptr interface{}) {
	err := json.Unmarshal(data, vptr)
	if nil != err {
		log.Println("Unmarshal:", err)
	}
}
