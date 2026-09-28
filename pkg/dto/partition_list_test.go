package dto

import (
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func partitions(begin, count int) model.PartitionRange {
	return model.PartitionRange{Begin: begin, Count: count}
}

func TestPartitionList_ToModel(t *testing.T) {
	tests := []struct {
		name string
		list PartitionList
		want model.PartitionList
	}{
		{
			name: "ids",
			list: "0,100,4095",
			want: model.PartitionList{partitions(0, 1), partitions(100, 1), partitions(4095, 1)},
		},
		{
			name: "ranges",
			list: "0-1,100-50,4095-1",
			want: model.PartitionList{partitions(0, 1), partitions(100, 50), partitions(4095, 1)},
		},
		{
			name: "all partitions",
			list: "0-4096",
			want: model.PartitionList{partitions(0, 4096)},
		},
		{
			name: "adjacent ranges",
			list: "0-100,100-100",
			want: model.PartitionList{partitions(0, 100), partitions(100, 100)},
		},
		{
			name: "listed order is kept",
			list: "200,0-100,100",
			want: model.PartitionList{partitions(200, 1), partitions(0, 100), partitions(100, 1)},
		},
		{
			name: "leading zeros",
			list: "007",
			want: model.PartitionList{partitions(7, 1)},
		},
		{
			name: "digest selects its partition",
			list: "EjRWeJq83vEjRRI0VniavN7xI0U=",
			want: model.PartitionList{partitions(1042, 1)},
		},
		{
			name: "empty",
			list: "",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.list.toModel()

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			require.NoError(t, tt.list.Validate())
		})
	}
}

func TestNewPartitionList(t *testing.T) {
	tests := []struct {
		name   string
		ranges model.PartitionList
		want   PartitionList
	}{
		{
			name:   "ids and ranges",
			ranges: model.PartitionList{partitions(0, 100), partitions(200, 1), partitions(4095, 1)},
			want:   "0-100,200,4095",
		},
		{
			name:   "empty",
			ranges: nil,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := newPartitionList(tt.ranges)

			assert.Equal(t, tt.want, list)
			got, err := list.toModel()
			require.NoError(t, err)
			assert.Equal(t, tt.ranges, got)
		})
	}
}

func TestPartitionList_Validate(t *testing.T) {
	tests := []struct {
		name    string
		list    PartitionList
		wantErr string
	}{
		{name: "space after comma", list: "0, 100", wantErr: "whitespace"},
		{name: "leading space", list: " 5", wantErr: "whitespace"},
		{name: "trailing space", list: "5 ", wantErr: "whitespace"},
		{name: "tab", list: "0,\t100", wantErr: "whitespace"},
		{name: "space inside a range", list: "0 -10", wantErr: "whitespace"},
		{name: "empty entry", list: "100,,200", wantErr: "failed to parse partition filter"},
		{name: "trailing comma", list: "100,", wantErr: "failed to parse partition filter"},
		{name: "id out of range", list: "4096", wantErr: "4096 exceeds the last partition 4095"},
		{name: "range out of range", list: "4095-2", wantErr: "4095-2 exceeds the last partition 4095"},
		{name: "range with zero count", list: "100-0", wantErr: "failed to parse partition filter"},
		{name: "double dash", list: "100--200", wantErr: "failed to parse partition filter"},
		{name: "explicit plus sign", list: "+5", wantErr: "failed to parse partition filter"},
		{name: "four digits with a leading zero", list: "0100", wantErr: "failed to parse partition filter"},
		{name: "duplicate id", list: "5,5", wantErr: "5 overlaps 5"},
		{name: "id inside a range", list: "0-10,5", wantErr: "5 overlaps 0-10"},
		{name: "overlapping ranges", list: "100-50,0-101", wantErr: "100-50 overlaps 0-101"},
		{name: "range covering a range", list: "0-4096,10-2", wantErr: "10-2 overlaps 0-4096"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ErrorContains(t, tt.list.Validate(), tt.wantErr)
		})
	}
}

func TestBackupRoutine_Validate_PartitionList(t *testing.T) {
	routine := &BackupRoutine{
		IntervalCron:  "@daily",
		SourceCluster: "cluster",
		Storage:       "storage",
		Namespaces:    &[]NamespaceName{"test"},
		PartitionList: "0-10,5",
	}

	err := routine.Validate()

	require.ErrorIs(t, err, errInvalidValue)
	assert.ErrorContains(t, err, `partition-list "0-10,5": 5 overlaps 0-10`)
}
