package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntityErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "a named entity reads as a sentence",
			err:  NotFound("routine", "daily"),
			want: `routine "daily" not found`,
		},
		{
			name: "a numeric identifier prints bare, not as a rune literal",
			err:  NotFound("job", RestoreJobID(65)),
			want: `job 65 not found`,
		},
		{
			name: "a clash names the entity",
			err:  AlreadyExists("storage", "s3-backup"),
			want: `storage "s3-backup" already exists`,
		},
		{
			name: "a reference names the holder",
			err:  InUse("policy", "keep-7", `it is used in routine "daily"`),
			want: `policy "keep-7" is in use: it is used in routine "daily"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

// The cause survives wrapping, so a caller several layers up still reports the right status.
func TestEntityErrorUnwrapsToItsCause(t *testing.T) {
	err := fmt.Errorf("failed to update configuration: %w", NotFound("routine", "daily"))

	require.ErrorIs(t, err, ErrNotFound)
	require.NotErrorIs(t, err, ErrAlreadyExists)
	require.NotErrorIs(t, err, ErrInUse)

	var entityErr *EntityError
	require.ErrorAs(t, err, &entityErr)
	assert.Equal(t, "routine", entityErr.Kind)
	assert.Equal(t, "daily", entityErr.Name)
}
