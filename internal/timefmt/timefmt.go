package timefmt

import "time"

var beijing = time.FixedZone("Asia/Shanghai", 8*60*60)

func UnixSeconds(t time.Time) int64 {
	return t.Unix()
}

func BeijingLocal(unixSeconds int64) string {
	if unixSeconds <= 0 {
		return ""
	}
	return time.Unix(unixSeconds, 0).In(beijing).Format("2006-01-02 15:04:05")
}
