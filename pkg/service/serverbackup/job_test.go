package serverbackup

import (
	"testing"
	"time"
)

// shortenLostJobTimeout shortens lostJobTimeout for the test. The test must not be parallel.
// Parallel tests start only after the sequential ones have ended, so none of them reads the
// variable while it is shortened.
func shortenLostJobTimeout(t *testing.T) {
	t.Helper()

	previous := lostJobTimeout
	lostJobTimeout = 1500 * time.Millisecond
	t.Cleanup(func() { lostJobTimeout = previous })
}
