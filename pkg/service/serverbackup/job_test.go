package serverbackup

import (
	"log/slog"
	"time"
)

var discardLogger = slog.New(slog.DiscardHandler)

// pollsUntilUnobserved is the number of failed polls that reaches unobservedJobTimeout.
const pollsUntilUnobserved = int(unobservedJobTimeout / pollInterval)

// pollsWithinTimeout is a number of failed polls that stays within it.
const pollsWithinTimeout = int(9 * time.Minute / pollInterval)
