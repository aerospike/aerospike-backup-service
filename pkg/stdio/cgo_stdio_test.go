package stdio

import (
	"os"
	"testing"
)

func TestCgoStdio_Capture(t *testing.T) {
	stdio := NewCgoStdio(true)
	const iterations = 100
	longStr := "Lorem ipsum dolor sit amet, consectetur adipiscing elit, " +
		"sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. " +
		"Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris " +
		"nisi ut aliquip ex ea commodo consequat. " +
		"Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. " +
		"Excepteur sint occaecat cupidatat non proident, " +
		"sunt in culpa qui officia deserunt mollit anim id est laborum. " // It's about 445 characters long

	// Repeat the string to reach your desired length
	printFunction := func() {
		_, _ = os.Stderr.WriteString(longStr)
	}
	for i := 0; i < iterations; i++ {
		captured := stdio.Capture(printFunction)

		if len(captured) == 0 {
			t.Errorf("no capture %d", i)
		}
		if captured != longStr {
			t.Errorf("captured string mismatch")
		}
	}
}
