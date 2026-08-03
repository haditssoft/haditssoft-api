package utils

func IndexOf[T comparable](slice []T, target T) int {
	for i, v := range slice {
		if v == target {
			return i
		}
	}
	return -1
}
