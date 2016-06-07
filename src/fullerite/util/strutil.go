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
	// all leading and trailing white space removed
	s = strings.TrimSpace(s)

	// this function used by strings.Map() where we change punctuation symbols NOT
	// allowed with the char '_' and all the other undesired chars with '=' because
	// the empty char cannot be used here; so these chars will be deleted later.
	translate := func(r rune) rune {
		if unicode.IsDigit(r) || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || unicode.IsSpace(r) {
			return r
		}
		if unicode.IsPunct(r) {
			if !allowPunctuation && !runeInSlice(r, allowedPunctuation) {
				return '_'
			}
			return r
		}
		return rune(-1)
	}
	s = strings.Map(translate, s)

	// Remove potential space
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
