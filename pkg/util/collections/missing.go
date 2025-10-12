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
