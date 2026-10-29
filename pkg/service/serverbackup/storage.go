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

// newRequestCommon describes the S3 location of a server job. It carries no credentials: each
// node resolves its own through the AWS default credential chain, with the storage's profile.
func newRequestCommon(storage model.Storage, namespace string) (infomodels.RequestCommon, error) {
	s3Storage, err := toS3Storage(storage)
	if err != nil {
		return infomodels.RequestCommon{}, err
	}

	return infomodels.RequestCommon{
		Namespace: namespace,
		Storage:   storageTypeS3,
		Path:      s3Storage.Path,
		Bucket:    s3Storage.Bucket,
		Region:    s3Storage.S3Region,
		Profile:   s3Storage.S3Profile,
		Endpoint:  s3Storage.S3EndpointOverride,
	}, nil
}
