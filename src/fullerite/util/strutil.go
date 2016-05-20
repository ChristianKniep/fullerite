package util

import (
	"strconv"
)

// StrToFloat converts a string value to float
func StrToFloat(val string) float64 {
	if i, err := strconv.ParseFloat(val, 64); err == nil {
		return i
	}
	return 0
}
