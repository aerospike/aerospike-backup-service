package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/aerospike/aerospike-backup-service/internal/server/dto"
	"github.com/gorilla/mux"
	"github.com/steinfletcher/apitest"
	"github.com/stretchr/testify/require"
)

func testConfigRestorePolicy() *dto.RestorePolicy {
	testIn32 := int32(10)
	return &dto.RestorePolicy{
		Parallel: &testIn32,
	}
}

func testRestoreRequest() dto.RestoreRequest {
	cluster := testConfigCluster()
	policy := testConfigRestorePolicy()
	storage := testConfigStorage()
	return dto.RestoreRequest{
		DestinationCuster: cluster,
		Policy:            policy,
		SourceStorage:     storage,
		SecretAgent:       nil,
	}
}

func testRestoreTimestampRequest() dto.RestoreTimestampRequest {
	cluster := testConfigCluster()
	policy := testConfigRestorePolicy()
	return dto.RestoreTimestampRequest{
		DestinationCuster: cluster,
		Policy:            policy,
		SecretAgent:       nil,
		Time:              time.Now().Unix(),
		Routine:           testRoutine,
	}
}

//nolint:dupl // No duplication here, just tests.
func TestService_RestoreFullHandler(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/v1/restore/full",
		h.RestoreFullHandler,
	).Methods(http.MethodPost)

	body := testRestoreRequest()
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	testCases := []struct {
		method     string
		statusCode int
		body       string
	}{
		{http.MethodPost, http.StatusAccepted, string(bodyBytes)},
		{http.MethodPost, http.StatusBadRequest, ""},
		{http.MethodGet, http.StatusMethodNotAllowed, string(bodyBytes)},
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

//nolint:dupl // No duplication here, just tests.
func TestService_RestoreIncrementalHandler(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/v1/restore/incremental",
		h.RestoreIncrementalHandler,
	).Methods(http.MethodPost)

	body := testRestoreRequest()
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	testCases := []struct {
		method     string
		statusCode int
		body       string
	}{
		{http.MethodPost, http.StatusAccepted, string(bodyBytes)},
		{http.MethodPost, http.StatusBadRequest, ""},
		{http.MethodGet, http.StatusMethodNotAllowed, string(bodyBytes)},
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
			URL("/v1/restore/incremental").
			Body(tt.body).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

//nolint:dupl // No duplication here, just tests.
func TestService_RestoreByTimeHandler(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/v1/restore/timestamp",
		h.RestoreByTimeHandler,
	).Methods(http.MethodPost)

	body := testRestoreTimestampRequest()
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	testCases := []struct {
		method     string
		statusCode int
		body       string
	}{
		{http.MethodPost, http.StatusAccepted, string(bodyBytes)},
		{http.MethodPost, http.StatusBadRequest, ""},
		{http.MethodGet, http.StatusMethodNotAllowed, string(bodyBytes)},
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
			URL("/v1/restore/timestamp").
			Body(tt.body).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_RestoreStatusHandler(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/v1/restore/status/{jobId}",
		h.RestoreStatusHandler,
	).Methods(http.MethodGet)

	const jobID = "1"

	testCases := []struct {
		method     string
		statusCode int
		jobID      string
	}{
		{http.MethodGet, http.StatusOK, jobID},
		{http.MethodGet, http.StatusNotFound, ""},
		{http.MethodGet, http.StatusBadRequest, "a"},
		{http.MethodPost, http.StatusMethodNotAllowed, jobID},
		{http.MethodConnect, http.StatusMethodNotAllowed, jobID},
		{http.MethodDelete, http.StatusMethodNotAllowed, jobID},
		{http.MethodPatch, http.StatusMethodNotAllowed, jobID},
		{http.MethodPut, http.StatusMethodNotAllowed, jobID},
		{http.MethodTrace, http.StatusMethodNotAllowed, jobID},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/v1/restore/status/%s", tt.jobID)).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_RetrieveConfig(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/v1/retrieve/configuration/{name}/{timestamp}",
		h.RetrieveConfig,
	).Methods(http.MethodGet)

	const (
		name      = "testConfig"
		timestamp = "1723453567"
	)

	testCases := []struct {
		method     string
		statusCode int
		name       string
		timestamp  string
	}{
		{http.MethodGet, http.StatusOK, name, timestamp},
		{http.MethodGet, http.StatusBadRequest, name, "a"},
		{http.MethodPost, http.StatusMethodNotAllowed, name, timestamp},
		{http.MethodConnect, http.StatusMethodNotAllowed, name, timestamp},
		{http.MethodDelete, http.StatusMethodNotAllowed, name, timestamp},
		{http.MethodPatch, http.StatusMethodNotAllowed, name, timestamp},
		{http.MethodPut, http.StatusMethodNotAllowed, name, timestamp},
		{http.MethodTrace, http.StatusMethodNotAllowed, name, timestamp},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/v1/retrieve/configuration/%s/%s", tt.name, tt.timestamp)).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}
