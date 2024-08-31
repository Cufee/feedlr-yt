package logic

import "strings"

func Substring(str string, end int) string {
	if len(str) <= end {
		return str
	}
	return strings.TrimSpace(str[:end])
}

func FirstString(input ...string) string {
	for _, v := range input {
		if v != "" {
			return v
		}
	}

	return ""
}
