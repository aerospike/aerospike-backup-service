package serverbackup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aerospike/backup-go/pkg/asinfo"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

// The kinds of server jobs, as errors name them.
const (
	backupJobKind  = "server backup"
	restoreJobKind = "server restore"
)

const (
	// pollInterval is how often a running server job is asked for its status.
	pollInterval = time.Second
	// cleanupTimeout bounds each wait made while cleaning up after a job: for an aborted job to
	// end, and for a namespace to open.
	cleanupTimeout = 30 * time.Second
)

// lostJobTimeout is how long a started server job may stay unreported before pollJob gives up on
// it. A new job can be invisible for a poll or two, and a job the cluster lost sight of can come
// back when the cluster settles. It is a variable so that tests can shorten it.
var lostJobTimeout = 30 * time.Second

// pollJob calls fetch for the status of the server job jobID every pollInterval and passes each
// status to settled. It returns the last status when settled returns true or an error. When fetch
// does not find the job, pollJob keeps polling until lostJobTimeout has passed since the job was
// last reported. Any other error of fetch ends the wait. kind names the job in errors.
func pollJob[S any](
	ctx context.Context,
	kind, jobID string,
	fetch func(context.Context) (S, error),
	settled func(S) (bool, error),
) (S, error) {
	var none S

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	lastSeen := time.Now()
	for {
		select {
		case <-ctx.Done():
			return none, ctx.Err()
		case <-ticker.C:
		}

		status, err := fetch(ctx)
		if errors.Is(err, asinfo.ErrNotFound) {
			if time.Since(lastSeen) > lostJobTimeout {
				return none, fmt.Errorf("%s %s not reported for %s: %w", kind, jobID, lostJobTimeout, err)
			}
			continue // a new job, or one the cluster lost sight of.
		}
		if err != nil {
			return none, fmt.Errorf("failed to get status of %s %s: %w", kind, jobID, err)
		}
		lastSeen = time.Now()

		done, err := settled(status)
		if err != nil || done {
			return status, err
		}
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
