package model

// IncrMode represents the mode for incremental backups.
type IncrMode string

const (
	// IncrModeDifferential is the default mode. It backs up data since the last successful backup (full or incremental).
	IncrModeDifferential IncrMode = "differential"
	// IncrModeCumulative backs up data since the last successful full backup.
	IncrModeCumulative IncrMode = "cumulative"
)

// String returns the string representation of the IncrMode.
func (m IncrMode) String() string {
	if m == "" {
		return string(IncrModeDifferential)
	}
	return string(m)
}
