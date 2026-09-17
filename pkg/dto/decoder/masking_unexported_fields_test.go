package decoder

import (
	"errors"
	"fmt"
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

	redacted := RedactSecrets(holder)
	assert.Equal(t, holder.String(), redacted.String())
}

// A slice of pointers to such a type keeps its elements' state too.
func TestRedactSecrets_PreservesUnexportedFieldsInSlice(t *testing.T) {
	keys := []*keyLike{{name: "routine", group: "backup"}}

	redacted := RedactSecrets(keys)
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

	redacted := RedactSecrets(envelope{
		Payload: &secretHolder{Password: literalPassword},
		Note:    "note",
	})

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
	redacted := RedactSecrets(envelope{Payload: &stateHolder{full: &now}})

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

// secretHolder is reached only through an unexported field and carries a credential in an
// exported one. Copying anything out of such a value is what reflect forbids, so the walk has
// to stop there rather than rebuild the struct around it.
type secretHolder struct {
	Password redact.Secret
}

type privatePath struct {
	inner secretHolder
}

// The walk never panics on a value it cannot copy out of, and never hands the credential back
// either: the unreachable field is dropped.
func TestRedactSecrets_UnexportedPathToASecret(t *testing.T) {
	require.NotPanics(t, func() {
		redacted := RedactSecrets(privatePath{inner: secretHolder{Password: literalPassword}})
		assert.NotEqual(t, redact.Secret(literalPassword), redacted.inner.Password)
	})
}

// The merge walk meets the same values and must not panic on them either.
func TestMergeSecrets_UnexportedPathToASecret(t *testing.T) {
	incoming := &privatePath{inner: secretHolder{Password: redact.Placeholder}}
	existing := &privatePath{inner: secretHolder{Password: literalPassword}}

	require.NotPanics(t, func() {
		assert.NoError(t, MergeSecrets(incoming, existing))
	})
}

// An error that carries no credential is handed back exactly as it is: its state is unexported,
// so rebuilding it would leave an error with an empty message.
func TestRedactSecrets_KeepsSecretFreeErrors(t *testing.T) {
	inner := errors.New("inner")
	wrapped := fmt.Errorf("connect cluster1: %w", inner)

	redacted, ok := RedactSecrets(wrapped).(error)
	require.True(t, ok)
	assert.Equal(t, wrapped.Error(), redacted.Error())
	assert.ErrorIs(t, redacted, inner)
}

// An error that does carry one is redacted like any other value, rather than being waved
// through: encoding/json publishes an error's fields, so handing it back as it stands would
// leak the literal into a marshaled response.
func TestRedactSecrets_RedactsErrorsCarryingASecret(t *testing.T) {
	redacted := RedactSecrets(&credentialCarryingError{User: "admin", Password: literalPassword})
	assert.Equal(t, "auth failed for admin", redacted.Error(), "the message survives")

	data, err := Marshal(redacted, JSON, false)
	require.NoError(t, err)
	assert.NotContains(t, string(data), literalPassword)
	assert.Contains(t, string(data), redact.Placeholder)
}

// credentialCarryingError is an error whose exported fields reach a credential.
type credentialCarryingError struct {
	User     string
	Password redact.Secret
}

func (e *credentialCarryingError) Error() string { return "auth failed for " + e.User }
