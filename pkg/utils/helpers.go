package utils

func Contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func Remove(list []string, s string) []string {
	if len(list) == 0 {
		return list
	}

	result := make([]string, 0, len(list))

	for _, v := range list {
		if v != s {
			result = append(result, v)
		}
	}

	return result
}

func AddUnique(list []string, s string) []string {
	if Contains(list, s) {
		return list
	}
	return append(list, s)
}
