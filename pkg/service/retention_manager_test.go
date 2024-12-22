package service

import (
	"context"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v2/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestMultipleBackups(t *testing.T) {
	dir := t.TempDir()
	s := &model.LocalStorage{Path: dir}
	routineName := "routine1"
	namespace := "ns1"
	backend := newBackend(routineName, s)

	for _, t := range []int64{10, 20, 30, 40, 50} {
		path := getFullPath(routineName, namespace, time.UnixMilli(t))
		metadata := model.BackupMetadata{Created: time.UnixMilli(t), Namespace: namespace}
		_ = backend.writeBackupMetadata(context.Background(), path, metadata)
	}

	for _, t := range []int64{22, 26, 32, 36, 42, 46, 52, 56} {
		path := getIncrementalPath(routineName, namespace, time.UnixMilli(t))
		metadata := model.BackupMetadata{Created: time.UnixMilli(t), Namespace: namespace}
		_ = backend.writeBackupMetadata(context.Background(), path, metadata)
	}

	r := NewBackupRetentionManager(
		backend,
		s,
		routineName,
		&model.RetentionPolicy{
			FullBackups: util.Ptr(3),
			IncrBackups: util.Ptr(2),
		},
	)

	ctx := context.Background()
	err := r.deleteOldBackups(ctx)
	require.NoError(t, err)

	full, _ := backend.FullBackupList(ctx, model.TimeBounds{})
	require.Equal(t, len(full), 3) // 30, 40, 50

	incr, _ := backend.IncrementalBackupList(ctx, model.TimeBounds{})
	require.Len(t, incr, 4) // 42, 46, 52, 56
}

func TestZeroRetentionLimit(t *testing.T) {
	dir := t.TempDir()
	s := &model.LocalStorage{Path: dir}
	routineName := "routine1"
	namespace := "ns1"
	backend := newBackend(routineName, s)

	for _, t := range []int64{10, 20, 30} {
		path := getFullPath(routineName, namespace, time.UnixMilli(t))
		metadata := model.BackupMetadata{Created: time.UnixMilli(t), Namespace: namespace}
		_ = backend.writeBackupMetadata(context.Background(), path, metadata)
	}

	for _, t := range []int64{22, 26, 32, 36, 42, 46, 52, 56} {
		path := getIncrementalPath(routineName, namespace, time.UnixMilli(t))
		metadata := model.BackupMetadata{Created: time.UnixMilli(t), Namespace: namespace}
		_ = backend.writeBackupMetadata(context.Background(), path, metadata)
	}

	r := NewBackupRetentionManager(
		backend,
		s,
		routineName,
		&model.RetentionPolicy{
			IncrBackups: util.Ptr(0),
		},
	)

	ctx := context.Background()
	err := r.deleteOldBackups(ctx)
	require.NoError(t, err)

	// all incremental backups should be deleted.

	full, _ := backend.FullBackupList(ctx, model.TimeBounds{})
	require.Equal(t, len(full), 3)

	incr, _ := backend.IncrementalBackupList(ctx, model.TimeBounds{})
	require.Empty(t, incr)
}

func TestNoBackups(t *testing.T) {
	dir := t.TempDir()
	s := &model.LocalStorage{Path: dir}
	routineName := "routine1"
	backend := newBackend(routineName, s)

	r := NewBackupRetentionManager(
		backend,
		s,
		routineName,
		&model.RetentionPolicy{
			FullBackups: util.Ptr(3),
			IncrBackups: util.Ptr(2),
		},
	)

	ctx := context.Background()
	err := r.deleteOldBackups(ctx)
	require.NoError(t, err)

	// nothing should be deleted as there are no backups.

	full, _ := backend.FullBackupList(ctx, model.TimeBounds{})
	require.Empty(t, full)

	incr, _ := backend.IncrementalBackupList(ctx, model.TimeBounds{})
	require.Empty(t, incr)
}

func TestExactRetentionLimit(t *testing.T) {
	dir := t.TempDir()
	s := &model.LocalStorage{Path: dir}
	routineName := "routine1"
	namespace := "ns1"
	backend := newBackend(routineName, s)

	for _, t := range []int64{10, 20, 30} {
		path := getFullPath(routineName, namespace, time.UnixMilli(t))
		metadata := model.BackupMetadata{Created: time.UnixMilli(t), Namespace: namespace}
		_ = backend.writeBackupMetadata(context.Background(), path, metadata)
	}

	for _, t := range []int64{32, 36} {
		path := getIncrementalPath(routineName, namespace, time.UnixMilli(t))
		metadata := model.BackupMetadata{Created: time.UnixMilli(t), Namespace: namespace}
		_ = backend.writeBackupMetadata(context.Background(), path, metadata)
	}

	r := NewBackupRetentionManager(
		backend,
		s,
		routineName,
		&model.RetentionPolicy{
			FullBackups: util.Ptr(3),
			IncrBackups: util.Ptr(2),
		},
	)

	ctx := context.Background()
	err := r.deleteOldBackups(ctx)
	require.NoError(t, err)

	// nothing should be deleted as retention limits are exactly met.

	full, _ := backend.FullBackupList(ctx, model.TimeBounds{})
	require.Len(t, full, 3)

	incr, _ := backend.IncrementalBackupList(ctx, model.TimeBounds{})
	require.Len(t, incr, 2)
}

func TestNilRetentionPolicy(t *testing.T) {
	dir := t.TempDir()
	s := &model.LocalStorage{Path: dir}
	routineName := "routine1"
	namespace := "ns1"
	backend := newBackend(routineName, s)

	for _, t := range []int64{10, 20, 30} {
		path := getFullPath(routineName, namespace, time.UnixMilli(t))
		metadata := model.BackupMetadata{Created: time.UnixMilli(t), Namespace: namespace}
		_ = backend.writeBackupMetadata(context.Background(), path, metadata)
	}

	r := NewBackupRetentionManager(
		backend,
		s,
		routineName,
		nil, // No retention policy
	)

	ctx := context.Background()
	err := r.deleteOldBackups(ctx)
	require.NoError(t, err)

	// Nothing should be deleted since the retention policy is nil.

	full, _ := backend.FullBackupList(ctx, model.TimeBounds{})
	require.Len(t, full, 3)
}

func TestHighRetentionLimits(t *testing.T) {
	dir := t.TempDir()
	s := &model.LocalStorage{Path: dir}
	routineName := "routine1"
	namespace := "ns1"
	backend := newBackend(routineName, s)

	for _, t := range []int64{10, 20, 30} {
		path := getFullPath(routineName, namespace, time.UnixMilli(t))
		metadata := model.BackupMetadata{Created: time.UnixMilli(t), Namespace: namespace}
		_ = backend.writeBackupMetadata(context.Background(), path, metadata)
	}

	r := NewBackupRetentionManager(
		backend,
		s,
		routineName,
		&model.RetentionPolicy{
			FullBackups: util.Ptr(10),
			IncrBackups: util.Ptr(10),
		},
	)

	ctx := context.Background()
	err := r.deleteOldBackups(ctx)
	require.NoError(t, err)

	// Retention limits are higher than the number of existing backups.
	// Nothing should be deleted.

	full, _ := backend.FullBackupList(ctx, model.TimeBounds{})
	require.Len(t, full, 3)
}
