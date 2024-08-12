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

const testStorage = "testStorage"

func testConfigStorage() dto.Storage {
	path := "/temp"
	return dto.Storage{
		Type:            "local",
		Path:            &path,
		MinPartSize:     5242880,
		MaxConnsPerHost: 1,
	}
}

func TestService_ConfigStorageActionHandlerPost(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/storage/{name}",
		h.ConfigStorageActionHandler,
	).Methods(http.MethodPost)

	const newStorageName = "newStorageName"

	body := testConfigStorage()
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	testCases := []struct {
		method     string
		statusCode int
		name       string
		body       string
	}{
		{http.MethodPost, http.StatusCreated, newStorageName, string(bodyBytes)},
		{http.MethodPost, http.StatusBadRequest, testStorage, string(bodyBytes)},
		{http.MethodPost, http.StatusBadRequest, newStorageName, ""},
		{http.MethodPost, http.StatusNotFound, "", string(bodyBytes)},
		{http.MethodGet, http.StatusMethodNotAllowed, newStorageName, string(bodyBytes)},
		{http.MethodConnect, http.StatusMethodNotAllowed, newStorageName, string(bodyBytes)},
		{http.MethodDelete, http.StatusMethodNotAllowed, newStorageName, string(bodyBytes)},
		{http.MethodPatch, http.StatusMethodNotAllowed, newStorageName, string(bodyBytes)},
		{http.MethodPut, http.StatusMethodNotAllowed, newStorageName, string(bodyBytes)},
		{http.MethodTrace, http.StatusMethodNotAllowed, newStorageName, string(bodyBytes)},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/config/storage/%s", tt.name)).
			Body(tt.body).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_ConfigStorageActionHandlerGet(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/storage/{name}",
		h.ConfigStorageActionHandler,
	).Methods(http.MethodGet)

	testCases := []struct {
		method     string
		statusCode int
		name       string
	}{
		{http.MethodGet, http.StatusOK, testStorage},
		{http.MethodGet, http.StatusNotFound, ""},
		{http.MethodPost, http.StatusMethodNotAllowed, testStorage},
		{http.MethodConnect, http.StatusMethodNotAllowed, testStorage},
		{http.MethodDelete, http.StatusMethodNotAllowed, testStorage},
		{http.MethodPatch, http.StatusMethodNotAllowed, testStorage},
		{http.MethodPut, http.StatusMethodNotAllowed, testStorage},
		{http.MethodTrace, http.StatusMethodNotAllowed, testStorage},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/config/storage/%s", tt.name)).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_ConfigStorageActionHandlerPut(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/storage/{name}",
		h.ConfigStorageActionHandler,
	).Methods(http.MethodPut)

	body := testConfigStorage()
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	testCases := []struct {
		method     string
		statusCode int
		name       string
		body       string
	}{
		{http.MethodPut, http.StatusOK, testStorage, string(bodyBytes)},
		{http.MethodPut, http.StatusBadRequest, testStorage, ""},
		{http.MethodPut, http.StatusNotFound, "", string(bodyBytes)},
		{http.MethodGet, http.StatusMethodNotAllowed, testStorage, string(bodyBytes)},
		{http.MethodConnect, http.StatusMethodNotAllowed, testStorage, string(bodyBytes)},
		{http.MethodDelete, http.StatusMethodNotAllowed, testStorage, string(bodyBytes)},
		{http.MethodPatch, http.StatusMethodNotAllowed, testStorage, string(bodyBytes)},
		{http.MethodPost, http.StatusMethodNotAllowed, testStorage, string(bodyBytes)},
		{http.MethodTrace, http.StatusMethodNotAllowed, testStorage, string(bodyBytes)},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/config/storage/%s", tt.name)).
			Body(tt.body).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_ConfigStorageActionHandlerDelete(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/storage/{name}",
		h.ConfigStorageActionHandler,
	).Methods(http.MethodDelete)

	testCases := []struct {
		method     string
		statusCode int
		name       string
	}{
		{http.MethodDelete, http.StatusNoContent, testStorage},
		{http.MethodDelete, http.StatusNotFound, ""},
		{http.MethodPost, http.StatusMethodNotAllowed, testStorage},
		{http.MethodConnect, http.StatusMethodNotAllowed, testStorage},
		{http.MethodGet, http.StatusMethodNotAllowed, testStorage},
		{http.MethodPatch, http.StatusMethodNotAllowed, testStorage},
		{http.MethodPut, http.StatusMethodNotAllowed, testStorage},
		{http.MethodTrace, http.StatusMethodNotAllowed, testStorage},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/config/storage/%s", tt.name)).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_ReadAllStorage(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/storage",
		h.ReadAllStorage,
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
			URL("/config/storage").
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}
