//go:build !time

package filter2

import (
	"time"
)

var nowTime = time.Time{}

func onTime() time.Time {
	return nowTime
}

// logtime 当编译时不包含 time 时，logTime 函数不会被编译
func logTime(s string, time time.Time) {

}
