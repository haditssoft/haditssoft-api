package utils

import "regexp"

func GetKitabName(kitabsNameAndNumber string) string {
	re := regexp.MustCompile(`[^a-zA-Z]`)
	return re.ReplaceAllString(kitabsNameAndNumber, "")
}
