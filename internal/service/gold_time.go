package service

import (
	"time"
)

const defaultGoldTZ = "Asia/Shanghai"

// GoldLocation returns Asia/Shanghai or configured IANA name.
func GoldLocation(tz string) *time.Location {
	name := defaultGoldTZ
	if t := trimTZ(tz); t != "" {
		name = t
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		loc, _ = time.LoadLocation(defaultGoldTZ)
	}
	return loc
}

func trimTZ(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// GoldNow returns current time in gold/welfare timezone.
func GoldNow(tz string) time.Time {
	return time.Now().In(GoldLocation(tz))
}

// GoldYYYYMM formats calendar month in gold timezone.
func GoldYYYYMM(t time.Time) string {
	return t.Format("200601")
}

// GoldYYYYMMDD formats calendar day in gold timezone.
func GoldYYYYMMDD(t time.Time) string {
	return t.Format("20060102")
}

// IsPenultimateExchangeDay true on the day before the last day of month (策划「月底前一天」).
func IsPenultimateExchangeDay(t time.Time) bool {
	loc := t.Location()
	y, m, day := t.Date()
	lastDay := time.Date(y, m+1, 0, 0, 0, 0, 0, loc).Day()
	return day == lastDay-1
}
