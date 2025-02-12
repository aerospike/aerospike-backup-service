//go:build !ci

package aerospike

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	as "github.com/aerospike/aerospike-client-go/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsNamespaceEmpty(t *testing.T) {
	cluster := model.NewLocalAerospikeCluster()
	client, aerr := as.NewClientWithPolicyAndHost(cluster.ASClientPolicy(), cluster.ASClientHosts()...)
	require.NoError(t, aerr)
	defer client.Close()

	nsValidator := &defaultNamespaceValidator{}

	namespace := "source-ns1"
	testSet := "test-set"

	insertTestData := func() {
		key, _ := as.NewKey(namespace, testSet, "test-key")
		bin := as.NewBin("test-bin", "test-value")
		_ = client.PutBins(nil, key, bin)
	}

	clearTestData := func() {
		key, _ := as.NewKey(namespace, testSet, "test-key")
		_, _ = client.Delete(nil, key)
	}

	tests := []struct {
		name      string
		sets      []string
		setup     func()
		want      bool
		wantError bool
	}{
		{
			name:  "empty namespace check",
			sets:  []string{},
			setup: clearTestData,
			want:  true,
		},
		{
			name:  "non-empty namespace check",
			sets:  []string{},
			setup: insertTestData,
			want:  false,
		},
		{
			name:  "empty set check",
			sets:  []string{testSet},
			setup: clearTestData,
			want:  true,
		},
		{
			name:  "non-empty set check",
			sets:  []string{testSet},
			setup: insertTestData,
			want:  false,
		},
		{
			name:  "non-existent set check",
			sets:  []string{"non-existent-set"},
			setup: clearTestData,
			want:  true,
		},
		{
			name:  "multiple sets check - all empty",
			sets:  []string{testSet, "another-test-set"},
			setup: clearTestData,
			want:  true,
		},
		{
			name:  "multiple sets check - one non-empty",
			sets:  []string{testSet, "another-test-set"},
			setup: insertTestData,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			got, err := nsValidator.IsEmpty(client, namespace, tt.sets)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
