package dateutil

import (
	"log"
	"time"
)

const (
	dateLayout = "20060102"
)

type DayType int

const (
	Workday DayType = iota
	Weekend
	Festival
)

func NextDay(date *time.Time) time.Time {
	return date.AddDate(0, 0, 1)
}

func ParseDateStr(date string) (time.Time, error) {
	return time.Parse(dateLayout, date)
}

func DateStr(date *time.Time) string {
	return date.Format(dateLayout)
}

func WeekdayToDayType(day time.Weekday) DayType {
	if time.Saturday == day || time.Sunday == day {
		return Weekend
	}
	return Workday
}

func DaysBetween(start, end string) (int, bool) {
	daySt, err1 := ParseDateStr(start)
	dayEd, err2 := ParseDateStr(end)
	if nil != err1 || nil != err2 {
		log.Printf("time parse errors: %s:%s, %s:%s\n", start, err1, end, err2)
		return 0, false
	}
	hours := dayEd.Sub(daySt).Hours()
	log.Println(daySt, dayEd, hours)
	return int(hours / 24), true
}
