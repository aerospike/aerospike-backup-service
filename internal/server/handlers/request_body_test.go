package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// An empty or whitespace-only body is rejected with 400 and the configuration is untouched;
// decoding it would yield an empty configuration and wipe every entity.
func TestRequestBody_RejectsEmptyBody(t *testing.T) {
	svc := newServiceWithNamespaceValidator(t)
	addValidBackupConfig(svc)

	for _, body := range []string{"", "  \n\t"} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/v1/config", strings.NewReader(body))
		w := httptest.NewRecorder()
		svc.UpdateConfig(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "request body is empty")
	}

	assert.Len(t, svc.config.Routines(), 1, "configuration must be untouched")
}

// Request bodies are bounded; an oversized body is rejected with 413 instead of being
// buffered and decoded in full.
func TestRequestBody_RejectsOversizedBody(t *testing.T) {
	svc := newServiceWithNamespaceValidator(t)
	addValidBackupConfig(svc)

	huge := `{"backup-routines":{"x":"` + strings.Repeat("a", maxRequestBodyBytes+1) + `"}}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/v1/config", strings.NewReader(huge))
	w := httptest.NewRecorder()
	svc.UpdateConfig(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code, w.Body.String())
	assert.Len(t, svc.config.Routines(), 1, "configuration must be untouched")
}
