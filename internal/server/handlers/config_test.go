package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aerospike/aerospike-backup-service/v2/internal/server/dto"
	"github.com/gorilla/mux"
	"github.com/steinfletcher/apitest"
	"github.com/stretchr/testify/require"
)

func testConfigDTO() *dto.Config {
	return &dto.Config{
		ServiceConfig: dto.BackupServiceConfig{
			HTTPServer: &dto.HTTPServerConfig{},
			Logger:     &dto.LoggerConfig{},
		},
		AerospikeClusters: make(map[string]*dto.AerospikeCluster),
		Storage:           make(map[string]*dto.Storage),
		BackupPolicies:    make(map[string]*dto.BackupPolicy),
	}
}

func TestService_ConfigActionHandlerGet(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config",
		h.ConfigActionHandler,
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
		{http.MethodTrace, http.StatusMethodNotAllowed},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL("/config").
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_ConfigActionHandlerPut(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config",
		h.ConfigActionHandler,
	).Methods(http.MethodPut)

	cfg := testConfigDTO()
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	testCases := []struct {
		method     string
		statusCode int
		body       string
	}{
		{http.MethodPut, http.StatusOK, string(cfgBytes)},
		{http.MethodPut, http.StatusBadRequest, ""},
		{http.MethodPost, http.StatusMethodNotAllowed, string(cfgBytes)},
		{http.MethodConnect, http.StatusMethodNotAllowed, string(cfgBytes)},
		{http.MethodDelete, http.StatusMethodNotAllowed, string(cfgBytes)},
		{http.MethodPatch, http.StatusMethodNotAllowed, string(cfgBytes)},
		{http.MethodTrace, http.StatusMethodNotAllowed, string(cfgBytes)},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL("/config").
			Body(tt.body).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}
