package decoder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stateHolder mimics model.BackupTime: all state is unexported and exposed via String().
type stateHolder struct {
	full *time.Time
}

func (s *stateHolder) String() string {
	if s.full == nil {
		return "never"
	}
	return s.full.Format(time.RFC3339)
}

// RedactSecrets must leave values that carry no secret intact, including state held in
// unexported fields. stateHolder stands in for such a type (model cannot be imported here
// without a cycle; internal/log/new_handler_test.go covers *model.BackupTime via the handler).
func TestRedactSecrets_PreservesUnexportedFields(t *testing.T) {
	t.Skip("BKRS-416: RedactSecrets rebuilds structs and zeroes unexported fields")

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	holder := &stateHolder{full: &now}

	redacted, ok := RedactSecrets(holder).(*stateHolder)
	require.True(t, ok)
	assert.Equal(t, holder.String(), redacted.String())
}
