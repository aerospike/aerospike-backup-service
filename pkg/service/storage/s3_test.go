package storage

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	connectivityTimeout = 1 * time.Second
}

func TestS3Storage_ConnectivitySuccess(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(s3ConnectivityHandler(false, false))
	t.Cleanup(ts.Close)

	s3Config := &model.S3Storage{
		Bucket:             "test-bucket",
		S3Region:           "us-east-1",
		S3EndpointOverride: ptr.Of(ts.URL),
		Auth: &model.S3Authentication{
			KeyIDSecret:     "key",
			AccessKeySecret: "secret",
		},
	}

	ctx := t.Context()
	accessor := NewS3StorageAccessor(ctx, secrets.NewResolver())

	_, err := accessor.getS3Client(ctx, s3Config)
	require.NoError(t, err)
}

func TestS3Storage_ConnectivityReadOnly(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(s3ConnectivityHandler(false, true))
	t.Cleanup(ts.Close)

	ctx := t.Context()
	accessor := NewS3StorageAccessor(ctx, secrets.NewResolver())

	_, err := accessor.getS3Client(ctx, &model.S3Storage{
		Bucket:             "test-bucket",
		S3Region:           "us-east-1",
		S3EndpointOverride: ptr.Of(ts.URL),
		Auth: &model.S3Authentication{
			KeyIDSecret:     "key",
			AccessKeySecret: "secret",
		},
	})
	require.NoError(t, err)
}

func TestS3Storage_ConnectivityFailure(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	ctx := t.Context()
	accessor := NewS3StorageAccessor(ctx, secrets.NewResolver())

	_, err := accessor.getS3Client(ctx, &model.S3Storage{
		Bucket:             "test-bucket",
		S3Region:           "us-east-1",
		S3EndpointOverride: ptr.Of(ts.URL),
		Auth: &model.S3Authentication{
			KeyIDSecret:     "key",
			AccessKeySecret: "secret",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connectivity check failed")
}

func s3ConnectivityHandler(denyList, denyWrite bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isProbe := strings.HasSuffix(r.URL.Path, connectivityProbeKey)

		switch {
		case r.Method == http.MethodHead && r.URL.Path == "/test-bucket":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/test-bucket" && r.URL.Query().Get("list-type") == "2":
			if denyList {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"></ListBucketResult>`))
		case r.Method == http.MethodPut && isProbe && r.URL.Query().Get("uploadId") != "":
			if denyWrite {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("ETag", `"test-etag"`)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && isProbe && r.URL.Query().Has("uploads"):
			if denyWrite {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<InitiateMultipartUploadResult>` +
				`<UploadId>test-upload-id</UploadId>` +
				`</InitiateMultipartUploadResult>`))
		case r.Method == http.MethodPost && isProbe && r.URL.Query().Get("uploadId") != "":
			if denyWrite {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<CompleteMultipartUploadResult><ETag>"test-etag"</ETag></CompleteMultipartUploadResult>`))
		case r.Method == http.MethodPut && isProbe:
			if denyWrite {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && isProbe:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}
