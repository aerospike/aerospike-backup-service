package serverbackup

import (
	"context"
	"fmt"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
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

// newRequestCommon describes the S3 location of a server job, with its credentials resolved.
func newRequestCommon(
	ctx context.Context,
	resolver secrets.Resolver,
	storage model.Storage,
	namespace string,
) (infomodels.RequestCommon, error) {
	s3Storage, err := toS3Storage(storage)
	if err != nil {
		return infomodels.RequestCommon{}, err
	}

	request := infomodels.RequestCommon{
		Namespace: namespace,
		Storage:   storageTypeS3,
		Bucket:    s3Storage.Bucket,
		Region:    s3Storage.S3Region,
		Profile:   s3Storage.S3Profile,
		Endpoint:  s3Storage.S3EndpointOverride,
	}

	if s3Storage.Auth == nil {
		return request, nil
	}

	request.AccessKey, err = resolver.Resolve(ctx, s3Storage.SecretAgent, s3Storage.Auth.KeyIDSecret)
	if err != nil {
		return infomodels.RequestCommon{}, fmt.Errorf("failed to resolve S3 access key ID: %w", err)
	}

	request.SecretKey, err = resolver.Resolve(ctx, s3Storage.SecretAgent, s3Storage.Auth.AccessKeySecret)
	if err != nil {
		return infomodels.RequestCommon{}, fmt.Errorf("failed to resolve S3 secret access key: %w", err)
	}

	return request, nil
}
