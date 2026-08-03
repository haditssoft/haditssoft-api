package utils

import "fmt"
import "strings"

func ArrayToString(a []string, delim string) string {
	return strings.Trim(strings.Replace(fmt.Sprint(a), " ", delim, -1), "[]")
}
