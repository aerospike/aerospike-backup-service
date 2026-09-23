package collections

import (
	"slices"
)

// MissingElements returns all elements in `subset` that are not present in `superset`.
func MissingElements(subset, superset []string) []string {
	var missing []string
	for _, element := range subset {
		if !slices.Contains(superset, element) {
			missing = append(missing, element)
		}
	}

	return missing
}

// FirstMatch returns the first element of b that also appears in a, or ("", false) if none.
func FirstMatch(a, b []string) (string, bool) {
	for _, v := range b {
		if slices.Contains(a, v) {
			return v, true
		}
	}

	return "", false
}

// Unique returns a new slice with duplicate elements removed, preserving order.
func Unique(s []string) []string {
	if len(s) == 0 {
		return s
	}

	seen := make(map[string]bool)
	unique := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			unique = append(unique, v)
		}
	}

	return unique
}
