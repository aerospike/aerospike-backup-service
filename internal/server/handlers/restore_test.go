package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aerospike/backup/internal/server/handlers/dto"
	"github.com/gorilla/mux"
	"github.com/steinfletcher/apitest"
	"github.com/stretchr/testify/require"
)

func testRestoreRequest() dto.RestoreRequest {
	cluster := testConfigCluster()
	policy := testConfigPolicy()
	storage := testConfigStorage()
	return dto.RestoreRequest{
		DestinationCuster: &cluster,
		Policy:            &policy,
		SourceStorage:     &storage,
		SecretAgent:       nil,
	}
}

func TestService_RestoreFullHandler(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/v1/restore/full",
		h.GetAllFullBackups,
	).Methods(http.MethodPost)

	body := testRestoreRequest()
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	testCases := []struct {
		method     string
		statusCode int
		body       string
	}{
		{http.MethodGet, http.StatusOK, string(bodyBytes)},
		{http.MethodGet, http.StatusBadRequest, ""},
		{http.MethodGet, http.StatusBadRequest, string(bodyBytes)},
		{http.MethodPost, http.StatusMethodNotAllowed, string(bodyBytes)},
		{http.MethodConnect, http.StatusMethodNotAllowed, string(bodyBytes)},
		{http.MethodDelete, http.StatusMethodNotAllowed, string(bodyBytes)},
		{http.MethodPatch, http.StatusMethodNotAllowed, string(bodyBytes)},
		{http.MethodPut, http.StatusMethodNotAllowed, string(bodyBytes)},
		{http.MethodTrace, http.StatusMethodNotAllowed, string(bodyBytes)},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL("/v1/restore/full").
			Body(tt.body).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}
