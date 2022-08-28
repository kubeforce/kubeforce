package strings

func Find(fn FilterFn, vars ...string) string {
	for _, s := range vars {
		if fn(s) {
			return s
		}
	}
	return ""
}

type FilterFn func(s string) bool

func IsNotEmpty(s string) bool {
	return s != ""
}

func Filter(fn FilterFn, vars ...string) []string {
	result := make([]string, 0, len(vars))
	for _, s := range vars {
		if fn(s) {
			result = append(result, s)
		}
	}
	return result
}
