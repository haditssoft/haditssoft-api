package utils

import (
	"strconv"
	"strings"
)

func GetKeyAndIndex(str string) (key string, index int, found bool) {
	var err error
	key = str
	before, after, got := strings.Cut(key, "[")
	if got {
		idx, _, got := strings.Cut(after, "]")
		if got {
			index, err = strconv.Atoi(idx)
			if err == nil {
				found = true
				key = before
			}
		}
	}
	return
}
