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

const testCluster = "testCluster"

func testSeedNode() dto.SeedNode {
	return dto.SeedNode{
		HostName: "host",
		Port:     3000,
		TLSName:  "tls",
	}
}

func testConfigCluster() *dto.AerospikeCluster {
	label := "label"
	timeout := int32(10)
	useAlternate := false
	queueSize := 1
	return &dto.AerospikeCluster{
		ClusterLabel:         &label,
		SeedNodes:            []dto.SeedNode{testSeedNode()},
		ConnTimeout:          &timeout,
		UseServicesAlternate: &useAlternate,
		Credentials:          &dto.Credentials{},
		TLS:                  &dto.TLS{},
		MaxParallelScans:     &queueSize,
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

	const newCluster = "newCluster"

	body := testConfigCluster()
	bodyBytes, err := json.Marshal(&body)
	require.NoError(t, err)

	testCases := []struct {
		method     string
		statusCode int
		name       string
		body       string
	}{
		{http.MethodPost, http.StatusCreated, newCluster, string(bodyBytes)},
		{http.MethodPost, http.StatusBadRequest, testCluster, ""},
		{http.MethodPost, http.StatusNotFound, "", string(bodyBytes)},
		{http.MethodGet, http.StatusMethodNotAllowed, newCluster, string(bodyBytes)},
		{http.MethodConnect, http.StatusMethodNotAllowed, newCluster, string(bodyBytes)},
		{http.MethodDelete, http.StatusMethodNotAllowed, newCluster, string(bodyBytes)},
		{http.MethodPatch, http.StatusMethodNotAllowed, newCluster, string(bodyBytes)},
		{http.MethodPut, http.StatusMethodNotAllowed, newCluster, string(bodyBytes)},
		{http.MethodTrace, http.StatusMethodNotAllowed, newCluster, string(bodyBytes)},
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

//nolint:dupl // No duplication here, just tests.
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

	body := testConfigCluster()
	bodyBytes, err := json.Marshal(&body)
	require.NoError(t, err)

	testCases := []struct {
		method     string
		statusCode int
		name       string
		body       string
	}{
		{http.MethodPut, http.StatusOK, testCluster, string(bodyBytes)},
		{http.MethodPut, http.StatusBadRequest, testCluster, ""},
		{http.MethodPut, http.StatusNotFound, "", string(bodyBytes)},
		{http.MethodGet, http.StatusMethodNotAllowed, testCluster, string(bodyBytes)},
		{http.MethodConnect, http.StatusMethodNotAllowed, testCluster, string(bodyBytes)},
		{http.MethodDelete, http.StatusMethodNotAllowed, testCluster, string(bodyBytes)},
		{http.MethodPatch, http.StatusMethodNotAllowed, testCluster, string(bodyBytes)},
		{http.MethodPost, http.StatusMethodNotAllowed, testCluster, string(bodyBytes)},
		{http.MethodTrace, http.StatusMethodNotAllowed, testCluster, string(bodyBytes)},
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

//nolint:dupl // No duplication here, just tests.
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
		{http.MethodDelete, http.StatusBadRequest, testCluster},
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

func TestService_ReadAerospikeClusters(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/clusters",
		h.ReadAerospikeClusters,
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
			URL("/config/clusters").
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}
