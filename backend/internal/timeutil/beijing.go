package timeutil

import "time"

// Beijing is GMT+8. All persisted timestamps and user-facing clocks use this zone.
var Beijing = time.FixedZone("CST", 8*60*60)

func Now() time.Time {
	return time.Now().In(Beijing)
}

func UnixMilli() int64 {
	return Now().UnixMilli()
}

func Format(t time.Time) string {
	return t.In(Beijing).Format("2006-01-02 15:04:05")
}

func Parse(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02 15:04:05", s, Beijing)
}

func DateLayout() string { return "2006-01-02 15:04:05" }

func StartOfDay(t time.Time) time.Time {
	t = t.In(Beijing)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, Beijing)
}

func EndOfDay(t time.Time) time.Time {
	return StartOfDay(t).Add(24*time.Hour - time.Nanosecond)
}

func WithinSameSecond(a, b time.Time) bool {
	return a.In(Beijing).Unix() == b.In(Beijing).Unix()
}

func FormatOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return Format(t)
}

func UnixMilliOf(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.In(Beijing).UnixMilli()
}
