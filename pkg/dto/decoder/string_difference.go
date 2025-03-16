package decoder

const maxStringsDifference = 2

// Find closest matching field.
// This will return an empty string if no close match was found.
func findSimilarField(field string, validFields []string) string {
	var bestMatch string
	minDistance := maxStringsDifference + 1 // Start with a value higher than maxDistance

	for _, validField := range validFields {
		distance := levenshteinDistance(field, validField)
		if distance == 1 {
			return validField // Cannot be improved upon
		}

		if distance > 0 && distance < minDistance { // Ignore exact matches
			minDistance = distance
			bestMatch = validField
		}
	}

	return bestMatch
}

// Helper functions to find how similar are two strings.
// 0 means identical, 1 means one character difference (added, skipped, swapped or replaced) etc.
// https://en.wikipedia.org/wiki/Damerau%E2%80%93Levenshtein_distance#Algorithm
func levenshteinDistance(s1, s2 string) int {
	if s1 == s2 {
		return 0
	}
	if len(s1) == 0 {
		return min(len(s2), maxStringsDifference+1)
	}
	if len(s2) == 0 {
		return min(len(s1), maxStringsDifference+1)
	}

	// Store only two rows
	prevRow := make([]int, len(s2)+1)
	currRow := make([]int, len(s2)+1)

	// Initialize first row
	for j := range prevRow {
		prevRow[j] = j
	}

	for i := 1; i <= len(s1); i++ {
		currRow[0] = i
		minInRow := maxStringsDifference + 1 // Track minimum in row for early exit

		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			currRow[j] = min(
				prevRow[j]+1,      // Deletion
				currRow[j-1]+1,    // Insertion
				prevRow[j-1]+cost, // Substitution
			)

			minInRow = min(minInRow, currRow[j])
		}

		// Important: we need to copy values, not just swap references
		copy(prevRow, currRow)

		if minInRow > maxStringsDifference {
			return maxStringsDifference + 1
		}
	}

	return min(prevRow[len(s2)], maxStringsDifference+1)
}
