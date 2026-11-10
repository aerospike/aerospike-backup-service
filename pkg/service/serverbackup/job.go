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

// pollInterval is how often a running server job is asked for its status.
const pollInterval = time.Second

// lostJobTimeout is how long a started server job may go unreported before it is taken for lost.
// A new job can be invisible for a poll or two, and one that dropped out of the cluster's view
// may come back as the cluster settles; past this, waiting longer would hold the cluster for
// nothing. A variable so that the tests can shorten it.
var lostJobTimeout = 30 * time.Second

// pollJob asks fetch for the status of the server job jobID every pollInterval, until settled
// reports that the wait is over or fails, and returns the last status. A job that fetch does not
// find is waited for, up to lostJobTimeout since it was last reported; any other failure of fetch
// ends the wait. job names the kind of the job in errors, e.g. "server backup".
func pollJob[S any](
	ctx context.Context,
	job, jobID string,
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
				return none, fmt.Errorf("%s %s not reported for %s: %w", job, jobID, lostJobTimeout, err)
			}
			continue // the job is not visible on the cluster yet.
		}
		if err != nil {
			return none, fmt.Errorf("failed to get %s status: %w", job, err)
		}
		lastSeen = time.Now()

		done, err := settled(status)
		if err != nil || done {
			return status, err
		}
	}
}

// errJobEnded reports a server job that ended without completing: outcome tells how, e.g.
// "failed", followed by what aborted the job and why, where the server tells.
func errJobEnded(job, jobID, outcome string, origin infomodels.AbortOrigin, reason string) error {
	var message strings.Builder
	fmt.Fprintf(&message, "%s %s %s", job, jobID, outcome)
	if origin != "" {
		fmt.Fprintf(&message, " (abort origin: %s)", origin)
	}
	if reason != "" {
		fmt.Fprintf(&message, ": %s", reason)
	}

	return errors.New(message.String())
}
