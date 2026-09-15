package handlers

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
)

// Numeric identifiers are printed as numbers (no rune literal); string names stay quoted.
func TestErrNotFound_QuotesOnlyStringNames(t *testing.T) {
	assert.Equal(t, `job 65 not found`, errNotFound("job", model.RestoreJobID(65)).Error())
	assert.Equal(t, `routine "r1" not found`, errNotFound("routine", "r1").Error())
}
