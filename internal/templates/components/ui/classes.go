package ui

import "strings"

func joinClasses(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		cleaned = append(cleaned, p)
	}
	return strings.Join(cleaned, " ")
}

func ifClass(condition bool, className string) string {
	if condition {
		return className
	}
	return ""
}

