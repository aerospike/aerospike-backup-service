package decoder

import (
	"reflect"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/redact"
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

// keyLike mimics quartz.JobKey: unexported strings and nothing to redact.
type keyLike struct {
	name  string
	group string
}

// RedactSecrets must leave values that carry no secret intact, including state held in
// unexported fields. stateHolder stands in for such a type (model cannot be imported here
// without a cycle; internal/log/new_handler_test.go covers *model.BackupTime via the handler).
func TestRedactSecrets_PreservesUnexportedFields(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	holder := &stateHolder{full: &now}

	redacted, ok := RedactSecrets(holder).(*stateHolder)
	require.True(t, ok)
	assert.Equal(t, holder.String(), redacted.String())
}

// A slice of pointers to such a type keeps its elements' state too.
func TestRedactSecrets_PreservesUnexportedFieldsInSlice(t *testing.T) {
	keys := []*keyLike{{name: "routine", group: "backup"}}

	redacted, ok := RedactSecrets(keys).([]*keyLike)
	require.True(t, ok)
	require.Len(t, redacted, 1)
	assert.Equal(t, *keys[0], *redacted[0])
}

// A secret reached through an interface is still redacted: the type of the enclosing value
// cannot answer for it, so the walk looks at the value behind the interface.
func TestRedactSecrets_RedactsThroughInterface(t *testing.T) {
	type secretHolder struct {
		Password redact.Secret
	}
	type envelope struct {
		Payload any
		Note    string
	}

	redacted, ok := RedactSecrets(envelope{
		Payload: &secretHolder{Password: literalPassword},
		Note:    "note",
	}).(envelope)
	require.True(t, ok)

	payload, ok := redacted.Payload.(*secretHolder)
	require.True(t, ok)
	assert.Equal(t, redact.Secret(redact.Placeholder), payload.Password)
	assert.Equal(t, "note", redacted.Note)
}

// A value behind an interface that holds no secret keeps its unexported state, even though
// the enclosing type alone cannot rule a secret out.
func TestRedactSecrets_PreservesUnexportedFieldsBehindInterface(t *testing.T) {
	type envelope struct {
		Payload any
	}

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	redacted, ok := RedactSecrets(envelope{Payload: &stateHolder{full: &now}}).(envelope)
	require.True(t, ok)

	payload, ok := redacted.Payload.(*stateHolder)
	require.True(t, ok)
	assert.Equal(t, "2026-01-02T03:04:05Z", payload.String())
}

// A type that reaches itself does not send the reachability check into a loop.
func TestContainsRedactable_HandlesRecursiveTypes(t *testing.T) {
	type node struct {
		Next   *node
		Secret redact.Secret
	}
	type plainNode struct {
		Next *plainNode
		Name string
	}

	assert.True(t, containsRedactable(reflect.TypeFor[*node]()))
	assert.False(t, containsRedactable(reflect.TypeFor[*plainNode]()))
}
