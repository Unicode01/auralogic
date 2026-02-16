package models

import "time"

// NowFunc 返回currentUTC时间
var NowFunc = func() time.Time {
	return time.Now().UTC()
}

