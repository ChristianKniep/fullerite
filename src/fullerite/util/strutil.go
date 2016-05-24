package util

import (
	"strconv"
	"strings"
)

// StrToFloat converts a string value to float
func StrToFloat(val string) float64 {
	if i, err := strconv.ParseFloat(val, 64); err == nil {
		return i
	}
	return 0
}

// StrSanitize enables handler lever sanitation
func StrSanitize(s string) string {
	s = strings.Replace(s, "=", "-", -1)
	s = strings.Replace(s, ":", "-", -1)
	return s
}
