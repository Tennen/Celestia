package client

import (
	"fmt"
	"time"
)

func timezoneOffset(timezone string) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return "+00:00"
	}
	_, offset := time.Now().In(loc).Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	return formatOffset(sign, offset)
}

func formatOffset(sign string, offsetSeconds int) string {
	hours := offsetSeconds / 3600
	minutes := (offsetSeconds % 3600) / 60
	return fmt.Sprintf("%s%02d:%02d", sign, hours, minutes)
}
