package logic

import "strings"

func Substring(str string, end int) string {
	if len(str) <= end {
		return str
	}
	return strings.TrimSpace(str[:end])
}
