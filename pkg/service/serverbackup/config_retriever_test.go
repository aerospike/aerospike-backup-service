package serverbackup

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestConfigRetriever(t *testing.T) {
	t.Parallel()

	_, err := NewConfigRetriever().RetrieveConfiguration(t.Context(), &model.BackupRoutine{}, time.Now())

	require.ErrorIs(t, err, model.ErrNotFound)
}
