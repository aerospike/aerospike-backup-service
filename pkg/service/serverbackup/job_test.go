package serverbackup

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var discardLogger = slog.New(slog.DiscardHandler)

// pollsUntilUnobserved is the number of failed polls that reaches unobservedJobTimeout.
const pollsUntilUnobserved = int(unobservedJobTimeout / pollInterval)

// pollsWithinTimeout is a number of failed polls that stays within it.
const pollsWithinTimeout = int(9 * time.Minute / pollInterval)

// TestPollJob_ReadCutShort checks that a wait ended while a status read retries names the read's
// error, and the end of the wait once.
func TestPollJob_ReadCutShort(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), cleanupTimeout)
		defer cancel()

		cause := errors.New("timeout")
		// A retried read returns the error of ctx joined with that of its last attempt.
		fetch := func(ctx context.Context) (struct{}, error) {
			<-ctx.Done()
			return struct{}{}, errors.Join(ctx.Err(), cause)
		}

		_, err := pollJob(ctx, discardLogger, backupJobKind, fullID, fetch, neverSettled)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.ErrorIs(t, err, cause)
		assert.Equal(t, 1, strings.Count(err.Error(), context.DeadlineExceeded.Error()))
	})
}

// TestPollJob_LogsUnobservedOnce checks that a job is logged once when its status is lost, not at
// every failed poll, and once when it is back.
func TestPollJob_LogsUnobservedOnce(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, nil))

		polls := 0
		fetch := func(context.Context) (struct{}, error) {
			polls++
			if polls <= pollsWithinTimeout {
				return struct{}{}, errors.New("timeout")
			}

			return struct{}{}, nil
		}
		settle := func(struct{}) (bool, error) { return true, nil }

		_, err := pollJob(t.Context(), logger, backupJobKind, fullID, fetch, settle)

		require.NoError(t, err)
		assert.Equal(t, 1, strings.Count(logs.String(), "No status of server backup"))
		assert.Equal(t, 1, strings.Count(logs.String(), "Status of server backup is back"))
	})
}

func neverSettled(struct{}) (bool, error) { return false, nil }
