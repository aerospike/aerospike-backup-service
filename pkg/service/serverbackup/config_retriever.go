package serverbackup

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/service/storage"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/collections"
)

// configFolder is the folder of a server backup the nodes upload their configuration files to.
const configFolder = "config"

// configRetriever reads the configuration files the nodes uploaded with a full backup.
type configRetriever struct {
	backupReader service.BackupReader
	operations   storage.Operations

	// Lock per routine. A retrieval read-locks it while it reads a backup's files, so that
	// retention deletes none of them meanwhile.
	routineStorage *collections.LockMap
}

var _ service.ConfigRetriever = (*configRetriever)(nil)

// NewConfigRetriever returns the ConfigRetriever of the server backup mode.
func NewConfigRetriever(
	backupReader service.BackupReader,
	operations storage.Operations,
	routineStorage *collections.LockMap,
) service.ConfigRetriever {
	return &configRetriever{backupReader: backupReader, operations: operations, routineStorage: routineStorage}
}

// RetrieveConfiguration returns the configuration files of the routine's last full backup at or
// before toTime, zipped under the names the nodes gave them: <node id>/<file name>.
func (r *configRetriever) RetrieveConfiguration(
	ctx context.Context, routine *model.BackupRoutine, toTime time.Time,
) ([]byte, error) {
	lock := r.routineStorage.Get(routine.Name)
	lock.RLock()
	defer lock.RUnlock()

	backups, err := r.backupReader.GetBackups(ctx, service.NewFullBackupFilter(routine).WithToTime(toTime).Last())
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve backups for routine %s: %w", routine.Name, err)
	}
	if len(backups) == 0 {
		return nil, fmt.Errorf("no full backups found before %v: %w", toTime, model.ErrNotFound)
	}

	folder := path.Join(backups[0].Key, configFolder)
	keys, err := r.operations.ReadFileNames(ctx, routine.Storage, folder, "", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list the configuration files in %s: %w", folder, err)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no configuration files found in %s: %w", folder, model.ErrNotFound)
	}

	archive := new(bytes.Buffer)
	writer := zip.NewWriter(archive)
	// The listed keys start with the storage path, which ReadFile adds itself.
	listed := path.Join(routine.Storage.GetPath(), folder) + "/"
	for _, key := range keys {
		name := strings.TrimPrefix(key, listed) // <node id>/<file name>
		file := path.Join(folder, name)
		content, err := r.operations.ReadFile(ctx, routine.Storage, file)
		if err != nil {
			return nil, fmt.Errorf("failed to read configuration file %s: %w", file, err)
		}

		entry, err := writer.Create(name)
		if err != nil {
			return nil, fmt.Errorf("failed to add %s to the archive: %w", file, err)
		}
		if _, err := entry.Write(content); err != nil {
			return nil, fmt.Errorf("failed to add %s to the archive: %w", file, err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close the archive: %w", err)
	}

	return archive.Bytes(), nil
}
