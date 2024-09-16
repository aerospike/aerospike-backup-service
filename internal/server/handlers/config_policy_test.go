package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v2/pkg/dto"
	"github.com/gorilla/mux"
	"github.com/steinfletcher/apitest"
	"github.com/stretchr/testify/require"
)

const testPolicy = "testPolicy"
const unusedTestPolicy = "unusedTestPolicy"

func testConfigBackupPolicy() *dto.BackupPolicy {
	testIn32 := int32(10)
	testInt := 10
	keepFiles := dto.KeepAll
	return &dto.BackupPolicy{
		RemoveFiles:   &keepFiles,
		Parallel:      &testInt,
		SocketTimeout: &testIn32,
		TotalTimeout:  &testIn32,
		MaxRetries:    &testIn32,
		RetryDelay:    &testIn32,
	}
}

func TestService_ConfigPolicyActionHandlerPost(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/policies/{name}",
		h.ConfigPolicyActionHandler,
	).Methods(http.MethodPost)

	const newTestPolicy = "newTestPolicy"

	body := testConfigBackupPolicy()
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	testCases := []struct {
		method     string
		statusCode int
		name       string
		body       string
	}{
		{http.MethodPost, http.StatusCreated, newTestPolicy, string(bodyBytes)},
		{http.MethodPost, http.StatusBadRequest, testPolicy, ""},
		{http.MethodPost, http.StatusNotFound, "", string(bodyBytes)},
		{http.MethodGet, http.StatusMethodNotAllowed, newTestPolicy, string(bodyBytes)},
		{http.MethodConnect, http.StatusMethodNotAllowed, newTestPolicy, string(bodyBytes)},
		{http.MethodDelete, http.StatusMethodNotAllowed, newTestPolicy, string(bodyBytes)},
		{http.MethodPatch, http.StatusMethodNotAllowed, newTestPolicy, string(bodyBytes)},
		{http.MethodPut, http.StatusMethodNotAllowed, newTestPolicy, string(bodyBytes)},
		{http.MethodTrace, http.StatusMethodNotAllowed, newTestPolicy, string(bodyBytes)},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/config/policies/%s", tt.name)).
			Body(tt.body).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

//nolint:dupl // No duplication here, just tests.
func TestService_ConfigPolicyActionHandlerGet(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/policies/{name}",
		h.ConfigPolicyActionHandler,
	).Methods(http.MethodGet)

	testCases := []struct {
		method     string
		statusCode int
		name       string
	}{
		{http.MethodGet, http.StatusOK, testPolicy},
		{http.MethodGet, http.StatusNotFound, ""},
		{http.MethodPost, http.StatusMethodNotAllowed, testPolicy},
		{http.MethodConnect, http.StatusMethodNotAllowed, testPolicy},
		{http.MethodDelete, http.StatusMethodNotAllowed, testPolicy},
		{http.MethodPatch, http.StatusMethodNotAllowed, testPolicy},
		{http.MethodPut, http.StatusMethodNotAllowed, testPolicy},
		{http.MethodTrace, http.StatusMethodNotAllowed, testPolicy},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/config/policies/%s", tt.name)).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_ConfigPolicyActionHandlerPut(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/policies/{name}",
		h.ConfigPolicyActionHandler,
	).Methods(http.MethodPut)

	body := testConfigBackupPolicy()
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	testCases := []struct {
		method     string
		statusCode int
		name       string
		body       string
	}{
		{http.MethodPut, http.StatusOK, testPolicy, string(bodyBytes)},
		{http.MethodPut, http.StatusBadRequest, testPolicy, ""},
		{http.MethodPut, http.StatusNotFound, "", string(bodyBytes)},
		{http.MethodGet, http.StatusMethodNotAllowed, testPolicy, string(bodyBytes)},
		{http.MethodConnect, http.StatusMethodNotAllowed, testPolicy, string(bodyBytes)},
		{http.MethodDelete, http.StatusMethodNotAllowed, testPolicy, string(bodyBytes)},
		{http.MethodPatch, http.StatusMethodNotAllowed, testPolicy, string(bodyBytes)},
		{http.MethodPost, http.StatusMethodNotAllowed, testPolicy, string(bodyBytes)},
		{http.MethodTrace, http.StatusMethodNotAllowed, testPolicy, string(bodyBytes)},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/config/policies/%s", tt.name)).
			Body(tt.body).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_ConfigPolicyActionHandlerDelete(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/policies/{name}",
		h.ConfigPolicyActionHandler,
	).Methods(http.MethodDelete)

	testCases := []struct {
		method     string
		statusCode int
		name       string
	}{
		{http.MethodDelete, http.StatusNoContent, unusedTestPolicy},
		{http.MethodDelete, http.StatusBadRequest, testPolicy},
		{http.MethodDelete, http.StatusNotFound, ""},
		{http.MethodPost, http.StatusMethodNotAllowed, testPolicy},
		{http.MethodConnect, http.StatusMethodNotAllowed, testPolicy},
		{http.MethodGet, http.StatusMethodNotAllowed, testPolicy},
		{http.MethodPatch, http.StatusMethodNotAllowed, testPolicy},
		{http.MethodPut, http.StatusMethodNotAllowed, testPolicy},
		{http.MethodTrace, http.StatusMethodNotAllowed, testPolicy},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/config/policies/%s", tt.name)).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_ReadPolicies(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/policies",
		h.ReadPolicies,
	).Methods(http.MethodGet)

	testCases := []struct {
		method     string
		statusCode int
	}{
		{http.MethodGet, http.StatusOK},
		{http.MethodPost, http.StatusMethodNotAllowed},
		{http.MethodConnect, http.StatusMethodNotAllowed},
		{http.MethodDelete, http.StatusMethodNotAllowed},
		{http.MethodPatch, http.StatusMethodNotAllowed},
		{http.MethodPut, http.StatusMethodNotAllowed},
		{http.MethodTrace, http.StatusMethodNotAllowed},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL("/config/policies").
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}
