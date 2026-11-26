package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/internal/attr"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

// The kinds of server jobs, as errors name them.
const (
	backupJobKind  = "server backup"
	restoreJobKind = "server restore"
)

const (
	// pollInterval is how often a running server restore job is asked for its status. Backup
	// jobs follow the JobPoller instead.
	pollInterval = time.Second
	// unobservedJobTimeout is how long a job may go without a status before pollJob gives up on
	// it. Giving up aborts the job, so the bound covers a transient loss of contact with the
	// cluster. It is a lower bound: it is checked when a status read returns, and a read retries.
	unobservedJobTimeout = 10 * time.Minute
	// cleanupTimeout bounds each wait made while cleaning up after a job: for an aborted job to
	// end, and for a namespace to open.
	cleanupTimeout = 30 * time.Second
)

// pollJob calls fetch for the status of the server job jobID every pollInterval and passes each
// status to settled. It returns the last status when settled returns true or an error. A failed
// fetch ends the wait only once unobservedJobTimeout has passed since the last status, or since
// polling began. kind names the job in errors and logs.
func pollJob[S any](
	ctx context.Context,
	logger *slog.Logger,
	kind, jobID string,
	fetch func(context.Context) (S, error),
	settled func(S) (bool, error),
) (S, error) {
	var none S

	logger = logger.With(slog.String("serverJobId", jobID))
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	lastSeen := time.Now()
	// fetchErr is nil while the job is observed.
	var fetchErr error
	for {
		select {
		case <-ctx.Done():
			return none, waitEnded(ctx, fetchErr)
		case <-ticker.C:
		}

		status, err := fetch(ctx)
		if err != nil && ctx.Err() != nil {
			return none, waitEnded(ctx, err)
		}
		if err != nil {
			if time.Since(lastSeen) > unobservedJobTimeout {
				return none, fmt.Errorf("%s %s not observed for %s: %w", kind, jobID, unobservedJobTimeout, err)
			}
			if fetchErr == nil {
				logger.Warn("No status of "+kind, attr.Error(err))
			}
			fetchErr = err
			continue
		}
		if fetchErr != nil {
			logger.Info("Status of " + kind + " is back")
		}
		lastSeen, fetchErr = time.Now(), nil

		done, err := settled(status)
		if err != nil || done {
			return status, err
		}
	}
}

// waitEnded returns the error of a wait that ctx ended, with the last failed status read as its
// cause.
func waitEnded(ctx context.Context, fetchErr error) error {
	switch {
	case fetchErr == nil:
		return ctx.Err()
	case errors.Is(fetchErr, ctx.Err()):
		return fetchErr // a read that ctx cut short already names both.
	default:
		return fmt.Errorf("%w, last status read: %w", ctx.Err(), fetchErr)
	}
}

// errJobEnded returns the error for a server job that ended without completing. outcome is
// "failed" or "was aborted". The abort origin and the reason are added when the server reports
// them.
func errJobEnded(kind, jobID, outcome string, origin infomodels.AbortOrigin, reason string) error {
	var message strings.Builder
	fmt.Fprintf(&message, "%s %s %s", kind, jobID, outcome)
	if origin != "" {
		fmt.Fprintf(&message, " (abort origin: %s)", origin)
	}
	if reason != "" {
		fmt.Fprintf(&message, ": %s", reason)
	}

	return errors.New(message.String())
}
