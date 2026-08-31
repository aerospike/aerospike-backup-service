package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBackupRoutine_Validate_Duplicates(t *testing.T) {
	tests := []struct {
		name    string
		routine *BackupRoutine
		wantErr string
	}{
		{
			name: "duplicate namespaces",
			routine: &BackupRoutine{
				SourceCluster: "c", Storage: "s", IntervalCron: "* * * * * *",
				Namespaces: &[]string{"ns1", "ns1"},
			},
			wantErr: "namespaces contains duplicate value",
		},
		{
			name: "duplicate set-list",
			routine: &BackupRoutine{
				SourceCluster: "c", Storage: "s", IntervalCron: "* * * * * *",
				Namespaces: &[]string{"ns1"},
				SetList:    []string{"set1", "set1"},
			},
			wantErr: "set-list contains duplicate value",
		},
		{
			name: "duplicate bin-list",
			routine: &BackupRoutine{
				SourceCluster: "c", Storage: "s", IntervalCron: "* * * * * *",
				Namespaces: &[]string{"ns1"},
				BinList:    []string{"bin1", "bin1"},
			},
			wantErr: "bin-list contains duplicate value",
		},
		{
			name: "duplicate rack-list",
			routine: &BackupRoutine{
				SourceCluster: "c", Storage: "s", IntervalCron: "* * * * * *",
				Namespaces: &[]string{"ns1"},
				RackList:   []int{1, 1},
			},
			wantErr: "rack-list contains duplicate value",
		},
		{
			name: "duplicate node-list",
			routine: &BackupRoutine{
				SourceCluster: "c", Storage: "s", IntervalCron: "* * * * * *",
				Namespaces: &[]string{"ns1"},
				NodeList:   []string{"node1", "node1"},
			},
			wantErr: "node-list contains duplicate value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.routine.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestRestorePolicy_Validate_Duplicates(t *testing.T) {
	tests := []struct {
		name    string
		policy  *RestorePolicy
		wantErr string
	}{
		{
			name: "duplicate set-list",
			policy: &RestorePolicy{
				BaseRestorePolicy: BaseRestorePolicy{
					SetList: []string{"set1", "set1"},
				},
			},
			wantErr: "set-list contains duplicate value",
		},
		{
			name: "duplicate bin-list",
			policy: &RestorePolicy{
				BaseRestorePolicy: BaseRestorePolicy{
					BinList: []string{"bin1", "bin1"},
				},
			},
			wantErr: "bin-list contains duplicate value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate(ValidationDefault)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestAerospikeCluster_Validate_Duplicates(t *testing.T) {
	tests := []struct {
		name    string
		cluster *AerospikeCluster
		wantErr string
	}{
		{
			name: "duplicate prefer-racks",
			cluster: &AerospikeCluster{
				SeedNodes:   []SeedNode{{HostName: "localhost", Port: 3000}},
				PreferRacks: []int{1, 1},
			},
			wantErr: "prefer-racks contains duplicate value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cluster.Validate(ValidationDefault)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestRateLimiterConfig_Validate_Duplicates(t *testing.T) {
	tests := []struct {
		name    string
		config  *RateLimiterConfig
		wantErr string
	}{
		{
			name: "duplicate white-list",
			config: &RateLimiterConfig{
				WhiteList: []string{"127.0.0.1", "127.0.0.1"},
			},
			wantErr: "white-list contains duplicate value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
