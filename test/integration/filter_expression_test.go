//go:build integration

package integration

import (
	"testing"
	"time"

	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupWithFilterExpression(t *testing.T) {
	filterExpression, asErr := as.ExpGreater(as.ExpIntBin("age"), as.ExpIntVal(25)).Base64()
	require.NoError(t, asErr)

	e := setupEnv(t, envOptions{
		filterExpression: filterExpression,
		setList:          setName,
	})

	seedRecords(t, e.asHost, e.asPort, []int{10, 20, 30, 40, 25})

	require.NoError(t, triggerFullBackup(t, e.server.URL, routineName))

	var backups []dto.BackupDetails
	var err error

	require.Eventually(t, func() bool {
		backups, err = fetchFullBackupsForRoutine(t, e.server.URL, routineName)
		return err == nil && len(backups) == 1
	}, 60*time.Second, 250*time.Millisecond)

	require.NoError(t, err)
	assert.Equal(t, namespace, backups[0].Namespace)
	assert.Equal(t, uint64(2), backups[0].RecordCount)
}

func seedRecords(t *testing.T, host string, port int, ages []int) {
	t.Helper()

	client, err := as.NewClient(host, port)
	require.NoError(t, err)
	t.Cleanup(client.Close)

	wp := as.NewWritePolicy(0, 0)
	for i, age := range ages {
		key, err := as.NewKey(namespace, setName, i)
		require.NoError(t, err)

		err = client.Put(wp, key, as.BinMap{"age": age})
		require.NoError(t, err)
	}
}
