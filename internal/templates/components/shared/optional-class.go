package shared

func OptionalClass(condition bool, class string) string {
	if condition {
		return class
	}
	return ""
}
