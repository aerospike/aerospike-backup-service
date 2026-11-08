package serverbackup

import (
	"fmt"

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

// withStorage returns common with the S3 location of storage. It carries no credentials: each
// node resolves its own through the AWS default credential chain, with the storage's profile.
func withStorage(common infomodels.RequestCommon, storage model.Storage) (infomodels.RequestCommon, error) {
	s3Storage, err := toS3Storage(storage)
	if err != nil {
		return infomodels.RequestCommon{}, err
	}

	common.Storage = storageTypeS3
	common.Path = s3Storage.Path
	common.Bucket = s3Storage.Bucket
	common.Region = s3Storage.S3Region
	common.Profile = s3Storage.S3Profile
	common.Endpoint = s3Storage.S3EndpointOverride

	return common, nil
}
