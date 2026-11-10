package serverbackup

import (
	"fmt"
	"path"
	"strings"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	infomodels "github.com/aerospike/backup-go/pkg/asinfo/models"
)

// storageTypeS3 is the only object store the server writes backups to.
const storageTypeS3 = "aws-s3"

// toS3Storage returns the storage as S3, the only kind a server backup can use.
func toS3Storage(storage model.Storage) (*model.S3Storage, error) {
	s3Storage, ok := storage.(*model.S3Storage)
	if !ok {
		return nil, fmt.Errorf("server backup requires S3 storage, got %T", storage)
	}

	return s3Storage, nil
}

// withStorage returns common with the S3 location of the routine's backups in storage: a folder
// of their own, as in scan mode, so that routines sharing a storage never see each other's
// backups. It carries no credentials: each node resolves its own through the AWS default
// credential chain, with the storage's profile.
func withStorage(
	common infomodels.RequestCommon,
	storage model.Storage,
	routineName string,
) (infomodels.RequestCommon, error) {
	s3Storage, err := toS3Storage(storage)
	if err != nil {
		return infomodels.RequestCommon{}, err
	}

	common.Storage = storageTypeS3
	common.Path = routinePath(s3Storage, routineName)
	common.Bucket = s3Storage.Bucket
	common.Region = s3Storage.S3Region
	common.Profile = s3Storage.S3Profile
	common.Endpoint = s3Storage.S3EndpointOverride

	return common, nil
}

// routinePath returns the key prefix of the routine's server backups in storage.
func routinePath(storage *model.S3Storage, routineName string) string {
	return path.Join(storage.Path, routineName)
}

// backupKey returns the key of a server backup: its routine's folder, then its job id. Restore by
// path takes the routine from the key's first segment, as it does for scan backups.
func backupKey(routineName, jobID string) string {
	return routineName + "/" + jobID
}

// parseBackupKey splits the key of a server backup into its routine's name and its job id.
func parseBackupKey(key string) (routineName, jobID string, err error) {
	routineName, jobID, ok := strings.Cut(key, "/")
	if !ok || routineName == "" {
		return "", "", fmt.Errorf("invalid server backup key %q: want <routine>/<job id>", key)
	}

	return routineName, jobID, nil
}
