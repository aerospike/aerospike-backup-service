//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupWithFilterExpression(t *testing.T) {
	filterExpression, asErr := as.ExpGreater(as.ExpIntBin("age"), as.ExpIntVal(25)).Base64()
	require.NoError(t, asErr)

	e := setupEnv(t, func(c *dto.Config) {
		r := testRoutine(c)
		r.SetList = []string{setName}
		r.FilterExpression = filterExpression
	})

	seedRecords(t, []int{10, 20, 30, 40, 25})

	triggerFullBackup(t, e.server.URL, routineName)

	backups := waitForFullBackupCount(t, e.server.URL, routineName, 1, 60*time.Second)

	assert.Equal(t, namespace, backups[0].Namespace)
	assert.Equal(t, uint64(2), backups[0].RecordCount)
}

func seedRecords(t *testing.T, ages []int) {
	t.Helper()

	writePolicy := as.NewWritePolicy(0, 0)
	for i, age := range ages {
		key, err := as.NewKey(namespace, setName, i)
		require.NoError(t, err)

		require.NoError(t, asClient.Put(writePolicy, key, as.BinMap{"age": age}))
	}
}
