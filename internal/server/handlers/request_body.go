package handlers

import (
	"bytes"
	"errors"
	"io"
	"net/http"

	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto"
	"github.com/aerospike/aerospike-backup-service/v3/pkg/dto/decoder"
)

// maxRequestBodyBytes bounds a JSON request body. The largest legitimate body is a whole
// configuration document, which stays far below this limit.
const maxRequestBodyBytes = 8 << 20 // 8 MiB

var errEmptyRequestBody = newStatusCodeError(errors.New("request body is empty"), http.StatusBadRequest)

// requestBody returns the request body bounded to maxRequestBodyBytes and rejects a body that
// carries no document at all (empty or whitespace only). Without that check an empty PUT /v1/config
// decodes to an empty configuration and wipes every configured entity.
func requestBody(w http.ResponseWriter, r *http.Request) (io.Reader, error) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
	if err != nil {
		return nil, errBadRequest(err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errEmptyRequestBody
	}

	return bytes.NewReader(data), nil
}

// decodeBody reads a bounded, non-empty request body into a T without validating it; request
// bodies are always JSON. On failure it writes the error response and returns false, so the
// handler just returns. Only dto.Config is read this way: its Validate must run after the handler
// merges stored secrets back into the incoming document. Every other payload validates itself, so
// use decodeBodyValidated.
func decodeBody[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	body, err := requestBody(w, r)
	if err != nil {
		httpError(w, err)
		return nil, false
	}

	value, err := dto.NewFromReader[T](body, decoder.JSON)
	if err != nil {
		httpError(w, errBadRequest(err))
		return nil, false
	}

	return value, true
}

// decodeBodyValidated decodes the request body into a T and validates it with T's own Validate.
func decodeBodyValidated[T any, PT interface {
	*T
	dto.Validator
}](w http.ResponseWriter, r *http.Request) (*T, bool) {
	value, ok := decodeBody[T](w, r)
	if !ok {
		return nil, false
	}

	if err := PT(value).Validate(); err != nil {
		httpError(w, errBadRequest(err))
		return nil, false
	}

	return value, true
}
