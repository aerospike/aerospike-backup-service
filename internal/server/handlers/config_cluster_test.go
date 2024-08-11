package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/aerospike/backup/internal/server/handlers/dto"
	"github.com/gorilla/mux"
	"github.com/steinfletcher/apitest"
	"github.com/stretchr/testify/require"
)

const testCluster = "testCluster"

func testSeedNode() dto.SeedNodeDTO {
	return dto.SeedNodeDTO{
		HostName: "host",
		Port:     3000,
		TLSName:  "tls",
	}
}

func testConfigCluster() dto.AerospikeClusterDTO {
	label := "label"
	timeout := int32(10)
	useAlternate := false
	queueSize := 1
	return dto.AerospikeClusterDTO{
		ClusterLabel:         &label,
		SeedNodes:            []dto.SeedNodeDTO{testSeedNode()},
		ConnTimeout:          &timeout,
		UseServicesAlternate: &useAlternate,
		Credentials:          &dto.CredentialsDTO{},
		TLS:                  &dto.TLSDTO{},
		ConnectionQueueSize:  &queueSize,
	}
}

func TestService_ConfigClusterActionHandlerPost(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/clusters/{name}",
		h.ConfigClusterActionHandler,
	).Methods(http.MethodPost)

	cfg := testConfigCluster()
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	testCases := []struct {
		method     string
		statusCode int
		name       string
		body       string
	}{
		{http.MethodPost, http.StatusCreated, testCluster, string(cfgBytes)},
		{http.MethodPost, http.StatusBadRequest, testCluster, ""},
		{http.MethodPost, http.StatusNotFound, "", string(cfgBytes)},
		{http.MethodGet, http.StatusMethodNotAllowed, testCluster, string(cfgBytes)},
		{http.MethodConnect, http.StatusMethodNotAllowed, testCluster, string(cfgBytes)},
		{http.MethodDelete, http.StatusMethodNotAllowed, testCluster, string(cfgBytes)},
		{http.MethodPatch, http.StatusMethodNotAllowed, testCluster, string(cfgBytes)},
		{http.MethodPut, http.StatusMethodNotAllowed, testCluster, string(cfgBytes)},
		{http.MethodTrace, http.StatusMethodNotAllowed, testCluster, string(cfgBytes)},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/config/clusters/%s", tt.name)).
			Body(tt.body).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_ConfigClusterActionHandlerGet(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/clusters/{name}",
		h.ConfigClusterActionHandler,
	).Methods(http.MethodGet)

	testCases := []struct {
		method     string
		statusCode int
		name       string
	}{
		{http.MethodGet, http.StatusOK, testCluster},
		{http.MethodGet, http.StatusNotFound, ""},
		{http.MethodPost, http.StatusMethodNotAllowed, testCluster},
		{http.MethodConnect, http.StatusMethodNotAllowed, testCluster},
		{http.MethodDelete, http.StatusMethodNotAllowed, testCluster},
		{http.MethodPatch, http.StatusMethodNotAllowed, testCluster},
		{http.MethodPut, http.StatusMethodNotAllowed, testCluster},
		{http.MethodTrace, http.StatusMethodNotAllowed, testCluster},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/config/clusters/%s", tt.name)).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_ConfigClusterActionHandlerPut(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/clusters/{name}",
		h.ConfigClusterActionHandler,
	).Methods(http.MethodPut)

	cfg := testConfigCluster()
	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	testCases := []struct {
		method     string
		statusCode int
		name       string
		body       string
	}{
		{http.MethodPut, http.StatusOK, testCluster, string(cfgBytes)},
		{http.MethodPut, http.StatusBadRequest, testCluster, ""},
		{http.MethodPut, http.StatusNotFound, "", string(cfgBytes)},
		{http.MethodGet, http.StatusMethodNotAllowed, testCluster, string(cfgBytes)},
		{http.MethodConnect, http.StatusMethodNotAllowed, testCluster, string(cfgBytes)},
		{http.MethodDelete, http.StatusMethodNotAllowed, testCluster, string(cfgBytes)},
		{http.MethodPatch, http.StatusMethodNotAllowed, testCluster, string(cfgBytes)},
		{http.MethodPost, http.StatusMethodNotAllowed, testCluster, string(cfgBytes)},
		{http.MethodTrace, http.StatusMethodNotAllowed, testCluster, string(cfgBytes)},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/config/clusters/%s", tt.name)).
			Body(tt.body).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_ConfigClusterActionHandlerDelete(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/clusters/{name}",
		h.ConfigClusterActionHandler,
	).Methods(http.MethodDelete)

	testCases := []struct {
		method     string
		statusCode int
		name       string
	}{
		{http.MethodDelete, http.StatusNoContent, testCluster},
		{http.MethodDelete, http.StatusNotFound, ""},
		{http.MethodPost, http.StatusMethodNotAllowed, testCluster},
		{http.MethodConnect, http.StatusMethodNotAllowed, testCluster},
		{http.MethodGet, http.StatusMethodNotAllowed, testCluster},
		{http.MethodPatch, http.StatusMethodNotAllowed, testCluster},
		{http.MethodPut, http.StatusMethodNotAllowed, testCluster},
		{http.MethodTrace, http.StatusMethodNotAllowed, testCluster},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/config/clusters/%s", tt.name)).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}
