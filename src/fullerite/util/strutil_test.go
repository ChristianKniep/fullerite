package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrSanitize(t *testing.T) {
	words := map[string]string{
		"string":                             "simple_string",
		"whitespace_string":                  "whitespace string",
		"empty_string":                       "",
		"unicode":                            "uni☃code☃",
		"non_ascii":                          "as\x81cii\x80",
		"stripped_to_empty_string":           "\n  \t\n",
		"non_ascii_stripped_to_empty_string": "\n\x81  \x80\t\n",
	}

	expected := map[string]string{
		"string":                             "simple_string",
		"whitespace_string":                  "whitespace_string",
		"empty_string":                       "null",
		"unicode":                            "unicode",
		"non_ascii":                          "ascii",
		"stripped_to_empty_string":           "null",
		"non_ascii_stripped_to_empty_string": "null",
	}

	for k := range words {
		assert.Equal(t, expected[k], StrSanitize(words[k], true, nil))
	}
}
