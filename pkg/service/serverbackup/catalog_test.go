package serverbackup

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/backup-go/pkg/server/lister"
	listermodels "github.com/aerospike/backup-go/pkg/server/lister/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCatalog_RoutineFolder checks that a routine's backups are listed and read under its own
// folder of the storage, so that routines sharing a storage never see each other's backups.
func TestCatalog_RoutineFolder(t *testing.T) {
	t.Parallel()

	storage := &model.S3Storage{Bucket: "bucket", Path: "backups"}
	bucket := fakeBucket{
		"backups/routine/" + fullID + "/metadata.json": metadataJSON(t, fullID, "bar"),
		"backups/other/" + incr1ID + "/metadata.json":  metadataJSON(t, incr1ID, "bar"),
	}
	c := &catalog{s3Client: func(context.Context, *model.S3Storage) (lister.S3API, error) { return bucket, nil }}

	t.Run("routine", func(t *testing.T) {
		t.Parallel()

		routine := &model.BackupRoutine{Name: "routine", Storage: storage}
		backups, err := c.GetBackups(t.Context(), service.NewFullBackupFilter(routine))
		require.NoError(t, err)
		require.Len(t, backups, 1, "only the routine's own backup")
		assert.Equal(t, "routine/"+fullID, backups[0].Key)
	})

	t.Run("path", func(t *testing.T) {
		t.Parallel()

		backups, err := c.GetBackups(t.Context(), service.NewPathFilter("routine/"+fullID, storage))
		require.NoError(t, err)
		require.Len(t, backups, 1)
		assert.Equal(t, "routine/"+fullID, backups[0].Key)

		// Another routine's job id is not found in this routine's folder.
		_, err = c.GetBackups(t.Context(), service.NewPathFilter("routine/"+incr1ID, storage))
		require.Error(t, err)
	})
}

// fakeBucket is an S3 bucket of objects by key, listed by folder as the lister lists it.
type fakeBucket map[string][]byte

func (b fakeBucket) ListObjectsV2(
	_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options),
) (*s3.ListObjectsV2Output, error) {
	prefix := aws.ToString(in.Prefix)
	seen := map[string]bool{}
	out := &s3.ListObjectsV2Output{}
	for key := range b {
		rest, ok := strings.CutPrefix(key, prefix)
		if !ok {
			continue
		}
		folder, _, _ := strings.Cut(rest, "/")
		if !seen[folder] {
			seen[folder] = true
			out.CommonPrefixes = append(out.CommonPrefixes, types.CommonPrefix{Prefix: aws.String(prefix + folder + "/")})
		}
	}

	return out, nil
}

func (b fakeBucket) GetObject(
	_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options),
) (*s3.GetObjectOutput, error) {
	data, ok := b[aws.ToString(in.Key)]
	if !ok {
		return nil, &types.NoSuchKey{}
	}

	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(data))}, nil
}

func metadataJSON(t *testing.T, jobID, namespace string) []byte {
	t.Helper()

	data, err := json.Marshal(listermodels.Metadata{
		BackupID: jobID, Namespace: namespace, Status: listermodels.BackupStatusComplete,
	})
	require.NoError(t, err)

	return data
}

func TestToBackupDetails(t *testing.T) {
	t.Parallel()

	storage := &model.S3Storage{Bucket: "bucket", Path: "backups"}
	jobTime := time.Date(2026, 10, 20, 9, 1, 11, 0, time.UTC)
	runStart := jobTime.Add(-time.Hour)

	tests := []struct {
		name        string
		metadata    listermodels.Metadata
		wantCreated time.Time
		wantFrom    time.Time
	}{
		{
			name:        "unsealed full is as of its job start",
			metadata:    listermodels.Metadata{BackupID: fullID, Namespace: "bar"},
			wantCreated: jobTime,
		},
		{
			// A namespace backed up after others starts its job well after the run: its data is
			// still as of the run's start, the next incremental's lower bound.
			name:        "sealed is as of its upper bound",
			metadata:    listermodels.Metadata{BackupID: fullID, Namespace: "bar", ModifiedBeforeUnixS: runStart.Unix()},
			wantCreated: runStart,
		},
		{
			name: "incremental",
			metadata: listermodels.Metadata{
				BackupID: fullID, Namespace: "bar", ModifiedAfterUnixS: runStart.Add(-time.Hour).Unix(),
			},
			wantCreated: jobTime,
			wantFrom:    runStart.Add(-time.Hour),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := toBackupDetails(tt.metadata, storage, "routine")

			assert.Equal(t, "routine/"+fullID, got.Key)
			assert.Equal(t, "bar", got.Namespace)
			assert.Equal(t, tt.wantCreated, got.Created)
			assert.Equal(t, time.UTC, got.Created.Location(), "times compare equal as map keys")
			assert.Equal(t, tt.wantFrom, got.From)
			assert.Equal(t, uint64(1), got.FileCount, "restore skips backups with no files")
		})
	}
}

func TestParseBackupKey(t *testing.T) {
	t.Parallel()

	routineName, jobID, err := parseBackupKey(backupKey("routine", fullID))
	require.NoError(t, err)
	assert.Equal(t, "routine", routineName)
	assert.Equal(t, fullID, jobID)

	for _, key := range []string{fullID, "/" + fullID} {
		_, _, err := parseBackupKey(key)
		require.Error(t, err, key)
	}
}

func TestRoutinePath(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "backups/routine", routinePath(&model.S3Storage{Path: "backups"}, "routine"))
	assert.Equal(t, "routine", routinePath(&model.S3Storage{}, "routine"), "a storage at the bucket root")
}
