package service

import (
	"fmt"
	"time"
)

const defaultActivityWeekTimezone = "Asia/Shanghai"

// ActivityWeekLocation 活跃榜周榜时区（周一 0 点切换）。
func ActivityWeekLocation() *time.Location {
	loc, err := time.LoadLocation(defaultActivityWeekTimezone)
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

// CurrentActivityWeekID 返回当前自然周标识 YYYY-Www（ISO 周，周一为一周起始）。
func CurrentActivityWeekID(now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	loc := ActivityWeekLocation()
	t := now.In(loc)
	y, w := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}
