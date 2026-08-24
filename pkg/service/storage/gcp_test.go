package storage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/model"
	secrets "github.com/aerospike/aerospike-backup-service/v3/pkg/service/secret"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGcpStorage_ConnectivitySuccess(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(gcpConnectivityHandler(false, false))
	t.Cleanup(ts.Close)

	ctx := t.Context()
	accessor := NewGcpStorageAccessor(secrets.NewResolver())

	_, err := accessor.getGcpClient(ctx, &model.GcpStorage{
		BucketName: "test-bucket",
		Endpoint:   ts.URL,
	})
	require.NoError(t, err)
}

func TestGcpStorage_ConnectivityReadOnly(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(gcpConnectivityHandler(false, true))
	t.Cleanup(ts.Close)

	ctx := t.Context()
	accessor := NewGcpStorageAccessor(secrets.NewResolver())

	_, err := accessor.getGcpClient(ctx, &model.GcpStorage{
		BucketName: "test-bucket",
		Endpoint:   ts.URL,
	})
	require.NoError(t, err)
}

func TestGcpStorage_ConnectivityFailure(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	ctx := connectivityFailureContext(t)
	accessor := NewGcpStorageAccessor(secrets.NewResolver())

	_, err := accessor.getGcpClient(ctx, &model.GcpStorage{
		BucketName: "test-bucket",
		Endpoint:   ts.URL,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connectivity check failed")
}

func gcpConnectivityHandler(denyList, denyWrite bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/b/test-bucket":
			handleGetBucket(w)
		case isTestPermissions(r):
			handleTestPermissions(w, r, denyList, denyWrite)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/b/test-bucket/o"):
			handleListObjects(w, denyList)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func handleGetBucket(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"name": "test-bucket"})
}

func isTestPermissions(r *http.Request) bool {
	return (r.Method == http.MethodPost || r.Method == http.MethodGet) &&
		strings.HasSuffix(r.URL.Path, "/iam/testPermissions")
}

func handleTestPermissions(w http.ResponseWriter, r *http.Request, denyList, denyWrite bool) {
	var requested []string
	if r.Method == http.MethodPost {
		var req struct {
			Permissions []string `json:"permissions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requested = req.Permissions
	} else {
		requested = r.URL.Query()["permissions"]
	}

	var granted []string
	for _, p := range requested {
		if p == "storage.objects.list" && denyList {
			continue
		}
		if (p == "storage.objects.create" || p == "storage.objects.delete") && denyWrite {
			continue
		}
		granted = append(granted, p)
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"kind":        "storage#testIamPermissionsResponse",
		"permissions": granted,
	})
}

func handleListObjects(w http.ResponseWriter, denyList bool) {
	if denyList {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"kind":"storage#objects"}`))
}
