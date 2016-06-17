package util

import (
	"strconv"
	"strings"
	"unicode"
)

// StrToFloat converts a string value to float
func StrToFloat(val string) float64 {
	if i, err := strconv.ParseFloat(val, 64); err == nil {
		return i
	}
	return 0
}

// StrSanitize enables handler lever sanitation
func StrSanitize(s string, allowPunctuation bool, allowedPunctuation []rune) string {
	// this function used by strings.Map() where we change punctuation symbols NOT
	// allowed with the char '_' and all the other undesired chars with '=' because
	// the empty char cannot be used here; so these chars will be deleted later.
	translate := func(r rune) rune {
		if r == ':' || r == '=' {
			return '-'
		}
		if unicode.IsDigit(r) || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			return r
		}
		if unicode.IsPunct(r) {
			if !allowPunctuation && !runeInSlice(r, allowedPunctuation) {
				return '_'
			}
			return r
		}
		if unicode.IsSpace(r) {
			return ' '
		}
		return rune(-1)
	}
	s = strings.Map(translate, s)

	// All leading and trailing white space removed
	s = strings.TrimSpace(s)

	// Change all spaces with the char '_'
	s = strings.Replace(s, " ", "_", -1)

	// If the value if empty just return null string
	if len(s) == 0 {
		return "null"
	}

	return s
}

func runeInSlice(a rune, list []rune) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// StringInSlice iterates over a slice checking if it contains a specific string
func StringInSlice(metricName string, collectorName string, list map[string]string) bool {
	for k, v := range list {
		if metricName == k && collectorName == v {
			return true
		}
	}
	return false
}
