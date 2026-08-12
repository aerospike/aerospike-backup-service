package dto

import (
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestDestinationClusterConfig_Validate(t *testing.T) {
	tests := []struct {
		name   string
		config DestinationClusterConfig
		err    error
	}{
		{
			name:   "empty config",
			config: DestinationClusterConfig{},
			err:    errRequiredEither,
		},
		{
			name: "both fields set",
			config: DestinationClusterConfig{
				Cluster: &AerospikeCluster{},
				Name:    "test-cluster",
			},
			err: errMutuallyExclusive,
		},
		{
			name: "only name set",
			config: DestinationClusterConfig{
				Name: "test-cluster",
			},
		},
		{
			name: "only cluster set",
			config: DestinationClusterConfig{
				Cluster: &AerospikeCluster{
					SeedNodes: []SeedNode{{HostName: "host", Port: 80}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.err != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDestinationClusterConfig_Validate_AllowEmpty(t *testing.T) {
	config := DestinationClusterConfig{}
	err := config.Validate(ValidationAllowEmpty)
	require.NoError(t, err)
}

func TestStorageConfig_Validate(t *testing.T) {
	tests := []struct {
		name          string
		storageConfig StorageConfig
		err           error
	}{
		{
			name:          "empty config",
			storageConfig: StorageConfig{},
			err:           errRequiredEither,
		},
		{
			name: "both fields set",
			storageConfig: StorageConfig{
				Storage: &Storage{},
				Name:    "test-storage",
			},
			err: errMutuallyExclusive,
		},
		{
			name: "only name set",
			storageConfig: StorageConfig{
				Name: "test-storage",
			},
		},
		{
			name: "only storage set",
			storageConfig: StorageConfig{
				Storage: &Storage{
					LocalStorage: &LocalStorage{
						Path: "test-path",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.storageConfig.Validate()
			if tt.err != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDestinationClusterConfig_ToModel(t *testing.T) {
	config := model.NewConfig()
	cluster := &model.AerospikeCluster{}
	_ = config.AddCluster("test-cluster", cluster)

	tests := []struct {
		name       string
		dstCluster DestinationClusterConfig
		want       *model.AerospikeCluster
		err        error
	}{
		{
			name: "convert from name",
			dstCluster: DestinationClusterConfig{
				Name: "test-cluster",
			},
			want: cluster,
		},
		{
			name: "convert from cluster config",
			dstCluster: DestinationClusterConfig{
				Cluster: &AerospikeCluster{
					ClusterLabel: "new cluster",
				},
			},
			want: &model.AerospikeCluster{
				ClusterLabel: "new cluster",
			},
		},
		{
			name: "non-existent cluster name",
			dstCluster: DestinationClusterConfig{
				Name: "non-existent",
			},
			err: errNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.dstCluster.ToModel(config)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want.Hash(), got.Hash())
		})
	}
}

func TestRestoreRequest_Validate(t *testing.T) {
	validCluster := &AerospikeCluster{
		SeedNodes: []SeedNode{{HostName: "host", Port: 80}},
	}
	validStorage := &Storage{
		LocalStorage: &LocalStorage{
			Path: "test-path",
		},
	}
	validPolicy := &RestorePolicy{}

	tests := []struct {
		name    string
		request RestoreRequest
		err     error
	}{
		{
			name: "missing backup path",
			request: RestoreRequest{
				DestinationClusterConfig: DestinationClusterConfig{
					Cluster: validCluster,
				},
				StorageConfig: StorageConfig{
					Storage: validStorage,
				},
				Policy: validPolicy,
			},
			err: errEmpty,
		},
		{
			name: "missing destination cluster",
			request: RestoreRequest{
				BackupDataPath: "test/path",
				StorageConfig: StorageConfig{
					Storage: validStorage,
				},
				Policy: validPolicy,
			},
			err: errRequiredEither,
		},
		{
			name: "missing storage",
			request: RestoreRequest{
				BackupDataPath: "test/path",
				DestinationClusterConfig: DestinationClusterConfig{
					Cluster: validCluster,
				},
				Policy: validPolicy,
			},
			err: errRequiredEither,
		},
		{
			name: "valid request with direct configs",
			request: RestoreRequest{
				BackupDataPath: "test/path",
				DestinationClusterConfig: DestinationClusterConfig{
					Cluster: validCluster,
				},
				StorageConfig: StorageConfig{
					Storage: validStorage,
				},
				Policy: validPolicy,
			},
		},
		{
			name: "valid request with names",
			request: RestoreRequest{
				BackupDataPath: "test/path",
				DestinationClusterConfig: DestinationClusterConfig{
					Name: "test-cluster",
				},
				StorageConfig: StorageConfig{
					Name: "test-storage",
				},
				Policy: validPolicy,
			},
		},
		{
			name: "non-local backup path",
			request: RestoreRequest{
				BackupDataPath: "../../outside",
				DestinationClusterConfig: DestinationClusterConfig{
					Cluster: validCluster,
				},
				StorageConfig: StorageConfig{
					Storage: validStorage,
				},
				Policy: validPolicy,
			},
			err: errValidation,
		},
		{
			name: "absolute backup path",
			request: RestoreRequest{
				BackupDataPath: "/outside",
				DestinationClusterConfig: DestinationClusterConfig{
					Cluster: validCluster,
				},
				StorageConfig: StorageConfig{
					Storage: validStorage,
				},
				Policy: validPolicy,
			},
			err: errValidation,
		},
		{
			name: "invalid secret agent override",
			request: RestoreRequest{
				BackupDataPath: "test/path",
				DestinationClusterConfig: DestinationClusterConfig{
					Cluster: validCluster,
				},
				StorageConfig: StorageConfig{
					Storage: validStorage,
				},
				SecretAgentConfig: &SecretAgentConfig{
					SecretAgent: &SecretAgent{
						ConnectionType: "tcp",
					},
				},
				Policy: validPolicy,
			},
			err: errEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRestoreTimestampRequest_Validate(t *testing.T) {
	validCluster := &AerospikeCluster{
		SeedNodes: []SeedNode{{HostName: "host", Port: 80}},
	}
	validPolicy := &TimestampRestorePolicy{}

	tests := []struct {
		name    string
		request RestoreTimestampRequest
		err     error
	}{
		{
			name: "missing time",
			request: RestoreTimestampRequest{
				DestinationClusterConfig: DestinationClusterConfig{
					Cluster: validCluster,
				},
				Policy:  validPolicy,
				Routine: "daily",
			},
			err: errEmpty,
		},
		{
			name: "negative time",
			request: RestoreTimestampRequest{
				Time: -100,
				DestinationClusterConfig: DestinationClusterConfig{
					Cluster: validCluster,
				},
				Policy:  validPolicy,
				Routine: "daily",
			},
			err: errNegative,
		},
		{
			name: "small time",
			request: RestoreTimestampRequest{
				Time: 100,
				DestinationClusterConfig: DestinationClusterConfig{
					Cluster: validCluster,
				},
				Policy:  validPolicy,
				Routine: "daily",
			},
			err: errValidation,
		},
		{
			name: "missing routine",
			request: RestoreTimestampRequest{
				DestinationClusterConfig: DestinationClusterConfig{
					Cluster: validCluster,
				},
				Policy: validPolicy,
				Time:   1739538000000,
			},
			err: errEmpty,
		},
		{
			name: "missing cluster",
			request: RestoreTimestampRequest{
				Time:    1739538000000,
				Policy:  validPolicy,
				Routine: "daily",
			},
		},
		{
			name: "valid request",
			request: RestoreTimestampRequest{
				DestinationClusterConfig: DestinationClusterConfig{
					Cluster: validCluster,
				},
				Policy:  validPolicy,
				Time:    1739538000000,
				Routine: "daily",
			},
		},
		{
			name: "storage only without cluster uses standard mode",
			request: RestoreTimestampRequest{
				StorageConfig: StorageConfig{
					Storage: &Storage{
						LocalStorage: &LocalStorage{Path: "test-path"},
					},
				},
				Policy:  validPolicy,
				Time:    1739538000000,
				Routine: "daily",
			},
		},
		{
			name: "valid runtime overrides",
			request: RestoreTimestampRequest{
				DestinationClusterConfig: DestinationClusterConfig{
					Cluster: validCluster,
				},
				StorageConfig: StorageConfig{
					Storage: &Storage{
						LocalStorage: &LocalStorage{Path: "test-path"},
					},
				},
				Policy:  validPolicy,
				Time:    1739538000000,
				Routine: "daily",
			},
		},
		{
			name: "invalid secret agent override",
			request: RestoreTimestampRequest{
				DestinationClusterConfig: DestinationClusterConfig{
					Cluster: validCluster,
				},
				SecretAgentConfig: &SecretAgentConfig{
					SecretAgent: &SecretAgent{
						ConnectionType: "tcp",
					},
				},
				Policy:  validPolicy,
				Time:    1739538000000,
				Routine: "daily",
			},
			err: errEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.err != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRestoreRequest_ToModel(t *testing.T) {
	config := model.NewConfig()
	cluster := &model.AerospikeCluster{
		ClusterLabel: "new cluster",
	}
	storage := &model.LocalStorage{Path: "test-path"}
	secretAgent := &model.SecretAgent{
		Address: "test-address",
	}
	clusterName := "test-cluster"
	_ = config.AddCluster(clusterName, cluster)
	storageName := "test-storage"
	_ = config.AddStorage(storageName, storage)

	tests := []struct {
		name    string
		request RestoreRequest
		want    *model.RestoreRequest
		err     error
	}{
		{
			name: "convert from names",
			request: RestoreRequest{
				DestinationClusterConfig: DestinationClusterConfig{
					Name: clusterName,
				},
				StorageConfig: StorageConfig{
					Name: storageName,
				},
				BackupDataPath: "test/path",
				Policy:         &RestorePolicy{},
			},
			want: &model.RestoreRequest{
				DestinationCluster: *cluster,
				SourceStorage:      storage,
				BackupDataPath:     "test/path",
				Policy:             model.RestorePolicy{},
			},
		},
		{
			name: "convert from values",
			request: RestoreRequest{
				DestinationClusterConfig: DestinationClusterConfig{
					Cluster: NewClusterFromModel(cluster, config.BackupConfigCopy()),
				},
				StorageConfig: StorageConfig{
					Storage: NewStorageFromModel(storage, config.BackupConfigCopy()),
				},
				SecretAgentConfig: &SecretAgentConfig{
					SecretAgent: newSecretAgentFromModel(secretAgent),
				},
				BackupDataPath: "test/path",
				Policy:         &RestorePolicy{},
			},
			want: &model.RestoreRequest{
				DestinationCluster: *cluster,
				SourceStorage:      storage,
				BackupDataPath:     "test/path",
				Policy:             model.RestorePolicy{},
			},
		},
		{
			name: "non-existent cluster",
			request: RestoreRequest{
				DestinationClusterConfig: DestinationClusterConfig{
					Name: "non-existent",
				},
				StorageConfig: StorageConfig{
					Name: storageName,
				},
				BackupDataPath: "test/path",
			},
			err: errNotFound,
		},
		{
			name: "non-existent storage",
			request: RestoreRequest{
				DestinationClusterConfig: DestinationClusterConfig{
					Name: clusterName,
				},
				StorageConfig: StorageConfig{
					Name: "non-existent",
				},
				BackupDataPath: "test/path",
			},
			err: errNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.request.ToModel(config)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			if tt.want != nil {
				require.Equal(t, tt.want.BackupDataPath, got.BackupDataPath)
				require.Equal(t, tt.want.DestinationCluster.Hash(), got.DestinationCluster.Hash())
			}
		})
	}
}

func TestRestoreTimestampRequest_ToModel(t *testing.T) {
	config := model.NewConfig()
	cluster := &model.AerospikeCluster{}
	_ = config.AddCluster("test-cluster", cluster)
	routineCluster := &model.AerospikeCluster{ClusterLabel: "routine-cluster"}
	routineStorage := &model.LocalStorage{Path: "routine-path"}
	routine := &model.BackupRoutine{
		Name:          "daily",
		SourceCluster: routineCluster,
		Storage:       routineStorage,
	}
	_ = config.AddRoutine(routine)
	_ = config.AddStorage("test-storage", &model.LocalStorage{Path: "test-path"})

	tests := []struct {
		name    string
		request RestoreTimestampRequest
		want    *model.RestoreTimestampRequest
		err     error
	}{
		{
			name: "valid conversion",
			request: RestoreTimestampRequest{
				DestinationClusterConfig: DestinationClusterConfig{
					Name: "test-cluster",
				},
				Time:    1739538000000,
				Routine: "daily",
				Policy:  &TimestampRestorePolicy{},
			},
			want: &model.RestoreTimestampRequest{
				DestinationCluster: *cluster,
				Time:               time.UnixMilli(1739538000000),
				RoutineName:        "daily",
				Storage:            routineStorage,
				Policy:             model.RestorePolicy{},
			},
		},
		{
			name: "non-existent cluster",
			request: RestoreTimestampRequest{
				DestinationClusterConfig: DestinationClusterConfig{
					Name: "non-existent",
				},
				Time:    1739538000000,
				Routine: "daily",
			},
			err: errNotFound,
		},
		{
			name: "non-existent routine",
			request: RestoreTimestampRequest{
				DestinationClusterConfig: DestinationClusterConfig{
					Name: "test-cluster",
				},
				Time:    1739538000000,
				Routine: "non-existent",
			},
			err: errNotFound,
		},
		{
			name: "storage override with routine fallback cluster",
			request: RestoreTimestampRequest{
				StorageConfig: StorageConfig{
					Name: "test-storage",
				},
				Time:    1739538000000,
				Routine: "daily",
				Policy:  &TimestampRestorePolicy{},
			},
			want: &model.RestoreTimestampRequest{
				DestinationCluster: *routineCluster,
				Time:               time.UnixMilli(1739538000000),
				RoutineName:        "daily",
				Storage:            &model.LocalStorage{Path: "test-path"},
				Policy:             model.RestorePolicy{},
			},
		},
		{
			name: "destination override with routine fallback storage",
			request: RestoreTimestampRequest{
				DestinationClusterConfig: DestinationClusterConfig{
					Name: "test-cluster",
				},
				Time:    1739538000000,
				Routine: "daily",
				Policy:  &TimestampRestorePolicy{},
			},
			want: &model.RestoreTimestampRequest{
				DestinationCluster: *cluster,
				Time:               time.UnixMilli(1739538000000),
				RoutineName:        "daily",
				Storage:            routineStorage,
				Policy:             model.RestorePolicy{},
			},
		},
		{
			name: "runtime overrides without local routine",
			request: RestoreTimestampRequest{
				DestinationClusterConfig: DestinationClusterConfig{
					Name: "test-cluster",
				},
				StorageConfig: StorageConfig{
					Name: "test-storage",
				},
				Time:    1739538000000,
				Routine: "cross-region-routine",
				Policy:  &TimestampRestorePolicy{},
			},
			want: &model.RestoreTimestampRequest{
				DestinationCluster: *cluster,
				Time:               time.UnixMilli(1739538000000),
				RoutineName:        "cross-region-routine",
				Storage:            &model.LocalStorage{Path: "test-path"},
				Policy:             model.RestorePolicy{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.request.ToModel(config)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			if tt.want != nil {
				require.Equal(t, tt.want.Time, got.Time)
				require.Equal(t, tt.want.RoutineName, got.RoutineName)
				require.Equal(t, tt.want.Storage, got.Storage)
				require.Equal(t, tt.want.DestinationCluster.Hash(), got.DestinationCluster.Hash())
			}
		})
	}
}
