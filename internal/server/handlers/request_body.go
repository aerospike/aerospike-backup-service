package handlers

import (
	"bufio"
	"errors"
	"io"
	"net/http"
	"unicode"

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
	body := bufio.NewReader(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))

	for {
		b, err := body.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, errEmptyRequestBody
			}

			return nil, errBadRequest(err)
		}

		if unicode.IsSpace(rune(b)) {
			continue
		}

		if err := body.UnreadByte(); err != nil {
			return nil, errBadRequest(err)
		}

		return body, nil
	}
}

// decodeBody reads a bounded, non-empty request body with read and returns the decoded value.
// On failure it writes the error response and returns false, so the handler just returns.
func decodeBody[T any](w http.ResponseWriter, r *http.Request, read func(io.Reader) (T, error)) (T, bool) {
	var zero T

	body, err := requestBody(w, r)
	if err != nil {
		httpError(w, err)
		return zero, false
	}

	value, err := read(body)
	if err != nil {
		httpError(w, errInvalidJSONPayload(err))
		return zero, false
	}

	return value, true
}

// jsonReader adapts a format-taking DTO reader to decodeBody; request bodies are always JSON.
func jsonReader[T any](read func(io.Reader, decoder.SerializationFormat) (T, error)) func(io.Reader) (T, error) {
	return func(r io.Reader) (T, error) {
		return read(r, decoder.JSON)
	}
}
