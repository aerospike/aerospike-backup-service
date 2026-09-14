package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A delay that does not fit in a time.Duration is rejected with 400 instead of wrapping
// negative and running the backup immediately.
func TestParseDelay_RejectsOverflow(t *testing.T) {
	_, err := parseDelay("9223372036854775807")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "should not exceed")

	delay, err := parseDelay("1500")
	require.NoError(t, err)
	assert.Equal(t, 1500, delay)
}
