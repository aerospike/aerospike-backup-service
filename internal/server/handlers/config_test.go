package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aerospike/backup/pkg/model"
	"github.com/gorilla/mux"
	"github.com/steinfletcher/apitest"
	"github.com/stretchr/testify/require"
)

func testConfigDTO() *model.Config {
	return &model.Config{
		ServiceConfig: &model.BackupServiceConfig{
			HTTPServer: &model.HTTPServerConfig{},
			Logger:     &model.LoggerConfig{},
		},
		AerospikeClusters: make(map[string]*model.AerospikeCluster),
		Storage:           make(map[string]*model.Storage),
		BackupPolicies:    make(map[string]*model.BackupPolicy),
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

func TestService_ApplyConfig(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/apply",
		h.ApplyConfig,
	).Methods(http.MethodPost)

	testCases := []struct {
		method     string
		statusCode int
	}{
		{http.MethodPost, http.StatusOK},
		{http.MethodGet, http.StatusMethodNotAllowed},
		{http.MethodPut, http.StatusMethodNotAllowed},
		{http.MethodConnect, http.StatusMethodNotAllowed},
		{http.MethodDelete, http.StatusMethodNotAllowed},
		{http.MethodPatch, http.StatusMethodNotAllowed},
		{http.MethodTrace, http.StatusMethodNotAllowed},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL("/config/apply").
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}
