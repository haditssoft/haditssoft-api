package utils

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func SnakeToFirstWordUpper(str string) string {
	strs := strings.Split(str, "_")
	for i, word := range strs {
		strs[i] = cases.Title(language.English, cases.Compact).String(word)
	}
	return strings.Join(strs[:], " ")
}
