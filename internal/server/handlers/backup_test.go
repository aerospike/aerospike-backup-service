package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gorilla/mux"
	"github.com/steinfletcher/apitest"
)

func TestService_GetAllFullBackups(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/backups/full",
		h.GetAllFullBackups,
	).Methods(http.MethodGet)

	const (
		from = "1723366610"
		to   = "1723366626"
	)

	testCases := []struct {
		method     string
		statusCode int
		from       string
		to         string
	}{
		{http.MethodGet, http.StatusOK, from, to},
		{http.MethodGet, http.StatusBadRequest, "a", "b"},
		{http.MethodGet, http.StatusBadRequest, to, from},
		{http.MethodPost, http.StatusMethodNotAllowed, from, to},
		{http.MethodConnect, http.StatusMethodNotAllowed, from, to},
		{http.MethodDelete, http.StatusMethodNotAllowed, from, to},
		{http.MethodPatch, http.StatusMethodNotAllowed, from, to},
		{http.MethodPut, http.StatusMethodNotAllowed, from, to},
		{http.MethodTrace, http.StatusMethodNotAllowed, from, to},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL("/backups/full").
			QueryParams(map[string]string{"from": tt.from, "to": tt.to}).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_GetFullBackupsForRoutine(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/backups/full/{name}",
		h.GetFullBackupsForRoutine,
	).Methods(http.MethodGet)

	const (
		from = "1723366610"
		to   = "1723366626"
		name = testRoutineName
	)

	testCases := []struct {
		method     string
		statusCode int
		from       string
		to         string
		name       string
	}{
		{http.MethodGet, http.StatusOK, from, to, name},
		{http.MethodGet, http.StatusNotFound, from, to, ""},
		{http.MethodGet, http.StatusBadRequest, "a", "b", name},
		{http.MethodGet, http.StatusBadRequest, to, from, name},
		{http.MethodPost, http.StatusMethodNotAllowed, from, to, name},
		{http.MethodConnect, http.StatusMethodNotAllowed, from, to, name},
		{http.MethodDelete, http.StatusMethodNotAllowed, from, to, name},
		{http.MethodPatch, http.StatusMethodNotAllowed, from, to, name},
		{http.MethodPut, http.StatusMethodNotAllowed, from, to, name},
		{http.MethodTrace, http.StatusMethodNotAllowed, from, to, name},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/backups/full/%s", tt.name)).
			QueryParams(map[string]string{"from": tt.from, "to": tt.to}).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_GetAllIncrementalBackups(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/backups/incremental",
		h.GetAllFullBackups,
	).Methods(http.MethodGet)

	const (
		from = "1723366610"
		to   = "1723366626"
	)

	testCases := []struct {
		method     string
		statusCode int
		from       string
		to         string
	}{
		{http.MethodGet, http.StatusOK, from, to},
		{http.MethodGet, http.StatusBadRequest, "a", "b"},
		{http.MethodGet, http.StatusBadRequest, to, from},
		{http.MethodPost, http.StatusMethodNotAllowed, from, to},
		{http.MethodConnect, http.StatusMethodNotAllowed, from, to},
		{http.MethodDelete, http.StatusMethodNotAllowed, from, to},
		{http.MethodPatch, http.StatusMethodNotAllowed, from, to},
		{http.MethodPut, http.StatusMethodNotAllowed, from, to},
		{http.MethodTrace, http.StatusMethodNotAllowed, from, to},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL("/backups/incremental").
			QueryParams(map[string]string{"from": tt.from, "to": tt.to}).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_GetIncrementalBackupsForRoutine(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/backups/incremental/{name}",
		h.GetIncrementalBackupsForRoutine,
	).Methods(http.MethodGet)

	const (
		from = "1723366610"
		to   = "1723366626"
		name = testRoutineName
	)

	testCases := []struct {
		method     string
		statusCode int
		from       string
		to         string
		name       string
	}{
		{http.MethodGet, http.StatusOK, from, to, name},
		{http.MethodGet, http.StatusNotFound, from, to, ""},
		{http.MethodGet, http.StatusBadRequest, "a", "b", name},
		{http.MethodGet, http.StatusBadRequest, to, from, name},
		{http.MethodPost, http.StatusMethodNotAllowed, from, to, name},
		{http.MethodConnect, http.StatusMethodNotAllowed, from, to, name},
		{http.MethodDelete, http.StatusMethodNotAllowed, from, to, name},
		{http.MethodPatch, http.StatusMethodNotAllowed, from, to, name},
		{http.MethodPut, http.StatusMethodNotAllowed, from, to, name},
		{http.MethodTrace, http.StatusMethodNotAllowed, from, to, name},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/backups/incremental/%s", tt.name)).
			QueryParams(map[string]string{"from": tt.from, "to": tt.to}).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}

func TestService_ScheduleFullBackup(t *testing.T) {
	t.Parallel()
	h := newServiceMock()
	router := mux.NewRouter()
	router.HandleFunc(
		"/backups/schedule/{name}",
		h.ScheduleFullBackup,
	).Methods(http.MethodGet)

	const (
		delay = "10"
		name  = testRoutineName
	)

	testCases := []struct {
		method     string
		statusCode int
		delay      string
		name       string
	}{
		{http.MethodGet, http.StatusOK, delay, name},
		{http.MethodGet, http.StatusNotFound, delay, ""},
		{http.MethodGet, http.StatusBadRequest, "b", name},
		{http.MethodGet, http.StatusBadRequest, "-10", name},
		{http.MethodGet, http.StatusBadRequest, delay, name},
		{http.MethodPost, http.StatusMethodNotAllowed, delay, name},
		{http.MethodConnect, http.StatusMethodNotAllowed, delay, name},
		{http.MethodDelete, http.StatusMethodNotAllowed, delay, name},
		{http.MethodPatch, http.StatusMethodNotAllowed, delay, name},
		{http.MethodPut, http.StatusMethodNotAllowed, delay, name},
		{http.MethodTrace, http.StatusMethodNotAllowed, delay, name},
	}

	for _, tt := range testCases {
		apitest.New().
			Handler(router).
			Method(tt.method).
			URL(fmt.Sprintf("/backups/schedule/%s", tt.name)).
			QueryParams(map[string]string{"delay": tt.delay}).
			Expect(t).
			Status(tt.statusCode).
			End()
	}
}
