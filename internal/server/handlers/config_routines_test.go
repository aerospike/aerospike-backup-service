package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/aerospike/backup/pkg/model"
	"github.com/gorilla/mux"
	"github.com/steinfletcher/apitest"
	"github.com/stretchr/testify/require"
)

const testRoutine = "testRoutine"

func testBackupRoutine() model.BackupRoutine {
	return model.BackupRoutine{
		BackupPolicy:  testPolicy,
		SourceCluster: testCluster,
		Storage:       testStorage,
		IntervalCron:  "0 0 * * * *",
	}
}

//nolint:dupl // No duplication here, just tests.
func TestService_ConfigRoutineActionHandlerPost(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/routines/{name}",
		h.ConfigRoutineActionHandler,
	).Methods(http.MethodPost)

	const newRoutineName = "newRoutine"

	body := testBackupRoutine()
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	testCases := []struct {
		method     string
		statusCode int
		name       string
		body       string
	}{
		{http.MethodPost, http.StatusCreated, newRoutineName, string(bodyBytes)},
		{http.MethodPost, http.StatusBadRequest, testRoutine, string(bodyBytes)},
		{http.MethodPost, http.StatusBadRequest, newRoutineName, ""},
		{http.MethodPost, http.StatusNotFound, "", string(bodyBytes)},
		{http.MethodGet, http.StatusMethodNotAllowed, newRoutineName, string(bodyBytes)},
		{http.MethodConnect, http.StatusMethodNotAllowed, newRoutineName, string(bodyBytes)},
		{http.MethodDelete, http.StatusMethodNotAllowed, newRoutineName, string(bodyBytes)},
		{http.MethodPatch, http.StatusMethodNotAllowed, newRoutineName, string(bodyBytes)},
		{http.MethodPut, http.StatusMethodNotAllowed, newRoutineName, string(bodyBytes)},
		{http.MethodTrace, http.StatusMethodNotAllowed, newRoutineName, string(bodyBytes)},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/config/routines/%s", tt.name)).
			Body(tt.body).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

//nolint:dupl // No duplication here, just tests.
func TestService_ConfigRoutineActionHandlerGet(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/routines/{name}",
		h.ConfigRoutineActionHandler,
	).Methods(http.MethodGet)

	testCases := []struct {
		method     string
		statusCode int
		name       string
	}{
		{http.MethodGet, http.StatusOK, testRoutine},
		{http.MethodGet, http.StatusNotFound, ""},
		{http.MethodPost, http.StatusMethodNotAllowed, testRoutine},
		{http.MethodConnect, http.StatusMethodNotAllowed, testRoutine},
		{http.MethodDelete, http.StatusMethodNotAllowed, testRoutine},
		{http.MethodPatch, http.StatusMethodNotAllowed, testRoutine},
		{http.MethodPut, http.StatusMethodNotAllowed, testRoutine},
		{http.MethodTrace, http.StatusMethodNotAllowed, testRoutine},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/config/routines/%s", tt.name)).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_ConfigRoutineActionHandlerPut(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/routines/{name}",
		h.ConfigRoutineActionHandler,
	).Methods(http.MethodPut)

	body := testBackupRoutine()
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	testCases := []struct {
		method     string
		statusCode int
		name       string
		body       string
	}{
		{http.MethodPut, http.StatusOK, testRoutine, string(bodyBytes)},
		{http.MethodPut, http.StatusBadRequest, testRoutine, ""},
		{http.MethodPut, http.StatusNotFound, "", string(bodyBytes)},
		{http.MethodGet, http.StatusMethodNotAllowed, testRoutine, string(bodyBytes)},
		{http.MethodConnect, http.StatusMethodNotAllowed, testRoutine, string(bodyBytes)},
		{http.MethodDelete, http.StatusMethodNotAllowed, testRoutine, string(bodyBytes)},
		{http.MethodPatch, http.StatusMethodNotAllowed, testRoutine, string(bodyBytes)},
		{http.MethodPost, http.StatusMethodNotAllowed, testRoutine, string(bodyBytes)},
		{http.MethodTrace, http.StatusMethodNotAllowed, testRoutine, string(bodyBytes)},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/config/routines/%s", tt.name)).
			Body(tt.body).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

//nolint:dupl // No duplication here, just tests.
func TestService_ConfigRoutineActionHandlerDelete(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/routines/{name}",
		h.ConfigRoutineActionHandler,
	).Methods(http.MethodDelete)

	testCases := []struct {
		method     string
		statusCode int
		name       string
	}{
		{http.MethodDelete, http.StatusNoContent, testRoutine},
		{http.MethodDelete, http.StatusNotFound, ""},
		{http.MethodPost, http.StatusMethodNotAllowed, testRoutine},
		{http.MethodConnect, http.StatusMethodNotAllowed, testRoutine},
		{http.MethodGet, http.StatusMethodNotAllowed, testRoutine},
		{http.MethodPatch, http.StatusMethodNotAllowed, testRoutine},
		{http.MethodPut, http.StatusMethodNotAllowed, testRoutine},
		{http.MethodTrace, http.StatusMethodNotAllowed, testRoutine},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/config/routines/%s", tt.name)).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_ReadRoutines(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/config/routines",
		h.ReadRoutines,
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
			URL("/config/routines").
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}
