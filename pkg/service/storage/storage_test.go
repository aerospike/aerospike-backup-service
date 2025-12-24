package storage

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/util/ptr"
	"github.com/stretchr/testify/assert"
)

func init() {
	connectivityTimeout = 100 * time.Millisecond
}

func TestGetS3Client_Connectivity(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" && r.URL.Path == "/test-bucket" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// Success case
	s3Config := &model.S3Storage{
		Bucket:             "test-bucket",
		S3Region:           "us-east-1",
		S3EndpointOverride: ptr.Of(ts.URL),
		Auth: &model.S3Authentication{
			KeyIDSecret:     "key",
			AccessKeySecret: "secret",
		},
	}

	accessor := NewS3StorageAccessor()
	client, err := accessor.getS3Client(t.Context(), s3Config)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Failure case
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failServer.Close()

	s3ConfigFail := &model.S3Storage{
		Bucket:             "test-bucket",
		S3Region:           "us-east-1",
		S3EndpointOverride: ptr.Of(failServer.URL),
		Auth: &model.S3Authentication{
			KeyIDSecret:     "key",
			AccessKeySecret: "secret",
		},
	}

	clientFail, err := accessor.getS3Client(t.Context(), s3ConfigFail)
	assert.Error(t, err)
	assert.Nil(t, clientFail)
	assert.Contains(t, err.Error(), "s3 storage connectivity check failed")
}

func TestGetGcpClient_Connectivity(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/b/test-bucket" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"name": "test-bucket"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// Success case
	gcpConfig := &model.GcpStorage{
		BucketName: "test-bucket",
		Endpoint:   ts.URL,
	}

	accessor := NewGcpStorageAccessor()
	client, err := accessor.getGcpClient(t.Context(), gcpConfig)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Failure case
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failServer.Close()

	gcpConfigFail := &model.GcpStorage{
		BucketName: "test-bucket",
		Endpoint:   failServer.URL,
	}

	clientFail, err := accessor.getGcpClient(t.Context(), gcpConfigFail)
	assert.Error(t, err)
	assert.Nil(t, clientFail)
	assert.Contains(t, err.Error(), "gcp storage connectivity check failed")
}

func TestGetAzureClient_Connectivity(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isAccountCheck := r.URL.Query().Get("comp") == "properties" && r.URL.Query().Get("restype") == "account"
		isContainerCheck := r.URL.Query().Get("restype") == "container"

		if r.Method == "GET" && (isAccountCheck || isContainerCheck) {
			w.Header().Set("x-ms-request-id", "req-id")
			w.Header().Set("x-ms-version", "2019-12-12")
			w.Header().Set("x-ms-sku-name", "Standard_LRS")
			w.Header().Set("x-ms-account-kind", "StorageV2")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	// Create a valid base64 key
	key := base64.StdEncoding.EncodeToString([]byte("dummy-key"))

	// Success case
	azureConfig := &model.AzureStorage{
		Endpoint: ts.URL,
		Auth: &model.AzureSharedKeyAuth{
			AccountName: "testaccount",
			AccountKey:  key,
		},
	}

	accessor := NewAzureStorageAccessor(t.Context())
	client, err := accessor.getAzureClient(t.Context(), azureConfig)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Failure case
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failServer.Close()

	azureConfigFail := &model.AzureStorage{
		Endpoint: failServer.URL,
		Auth: &model.AzureSharedKeyAuth{
			AccountName: "testaccount",
			AccountKey:  key,
		},
	}
	client, err = accessor.getAzureClient(t.Context(), azureConfigFail)

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "azure blob storage connectivity check failed")
}
