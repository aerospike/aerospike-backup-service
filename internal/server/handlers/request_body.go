package handlers

import (
	"bytes"
	"errors"
	"io"
	"net/http"

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

// dtoFromReader is the shape shared by every dto.NewXxxFromReader constructor.
type dtoFromReader[T any] func(io.Reader, decoder.SerializationFormat) (T, error)

// decodeBody reads a bounded, non-empty request body with read and returns the decoded value;
// request bodies are always JSON. On failure it writes the error response and returns false, so
// the handler just returns.
func decodeBody[T any](w http.ResponseWriter, r *http.Request, read dtoFromReader[T]) (T, bool) {
	var zero T

	body, err := requestBody(w, r)
	if err != nil {
		httpError(w, err)
		return zero, false
	}

	value, err := read(body, decoder.JSON)
	if err != nil {
		httpError(w, errBadRequest(err))
		return zero, false
	}

	return value, true
}
