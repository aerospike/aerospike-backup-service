package storage

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAzureStorage_ConnectivitySuccess(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(azureConnectivityHandler(false, false))
	t.Cleanup(ts.Close)

	key := base64.StdEncoding.EncodeToString([]byte("dummy-key"))

	ctx := t.Context()
	accessor := NewAzureStorageAccessor(secrets.NewResolver())

	_, err := accessor.getAzureClient(ctx, &model.AzureStorage{
		Endpoint:      ts.URL,
		ContainerName: "test-container",
		Auth: &model.AzureSharedKeyAuth{
			AccountName: "testaccount",
			AccountKey:  key,
		},
	})
	require.NoError(t, err)
}

func TestAzureStorage_ConnectivityReadOnly(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(azureConnectivityHandler(false, true))
	t.Cleanup(ts.Close)

	key := base64.StdEncoding.EncodeToString([]byte("dummy-key"))

	ctx := t.Context()
	accessor := NewAzureStorageAccessor(secrets.NewResolver())

	_, err := accessor.getAzureClient(ctx, &model.AzureStorage{
		Endpoint:      ts.URL,
		ContainerName: "test-container",
		Auth: &model.AzureSharedKeyAuth{
			AccountName: "testaccount",
			AccountKey:  key,
		},
	})
	require.NoError(t, err)
}

func TestAzureStorage_ConnectivityFailure(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	key := base64.StdEncoding.EncodeToString([]byte("dummy-key"))

	ctx := connectivityFailureContext(t)
	accessor := NewAzureStorageAccessor(secrets.NewResolver())

	_, err := accessor.getAzureClient(ctx, &model.AzureStorage{
		Endpoint:      ts.URL,
		ContainerName: "test-container",
		Auth: &model.AzureSharedKeyAuth{
			AccountName: "testaccount",
			AccountKey:  key,
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connectivity check failed")
}

func TestEndpointHasEmbeddedSAS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		endpoint string
		isSAS    bool
		wantErr  bool
	}{
		{
			name: "service SAS URL",
			endpoint: "https://account.blob.core.windows.net/" +
				"?sv=2020-08-04&ss=b&srt=sco&sp=r&se=2026-01-01T00:00:00Z&sig=abc123",
			isSAS: true,
		},
		{
			name:     "plain service URL",
			endpoint: "https://account.blob.core.windows.net/",
			isSAS:    false,
		},
		{
			name:     "invalid URL",
			endpoint: "://not-a-url",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sas, err := endpointHasEmbeddedSAS(tt.endpoint)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.isSAS, sas)
		})
	}
}

func azureConnectivityHandler(denyList, denyWrite bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAccountCheck := r.URL.Query().Get("comp") == "properties" && r.URL.Query().Get("restype") == "account"
		isContainerCheck := r.URL.Query().Get("restype") == "container" && r.URL.Query().Get("comp") != "list"
		isListBlobs := r.URL.Query().Get("comp") == "list" && r.URL.Query().Get("restype") == "container"
		isProbeBlob := strings.Contains(r.URL.Path, connectivityProbeKey)

		switch {
		case r.Method == http.MethodGet && (isAccountCheck || isContainerCheck):
			w.Header().Set("x-ms-version", "2019-12-12")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && isListBlobs:
			if denyList {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("x-ms-version", "2019-12-12")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><EnumerationResults></EnumerationResults>`))
		case r.Method == http.MethodPut && isProbeBlob:
			if denyWrite {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodDelete && isProbeBlob:
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}
}
