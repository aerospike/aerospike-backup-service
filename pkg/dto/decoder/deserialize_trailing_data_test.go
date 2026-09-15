package decoder

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Data after the JSON document is rejected instead of silently ignored.
func TestDeserialize_RejectsTrailingData(t *testing.T) {
	type doc struct {
		Bucket string `json:"bucket"`
	}

	var j doc
	err := Deserialize(&j, strings.NewReader(`{"bucket":"b"} this is not json`), JSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "after the JSON document")

	require.NoError(t, Deserialize(&j, strings.NewReader(`{"bucket":"b"}  `+"\n"), JSON))
	assert.Equal(t, "b", j.Bucket)
}
