package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testLogger struct {
	level   slog.Level
	msg     string
	attrs   []slog.Attr
	records int
}

func (l *testLogger) Handle(_ context.Context, r slog.Record) error {
	l.level = r.Level
	l.msg = r.Message
	r.Attrs(func(a slog.Attr) bool {
		l.attrs = append(l.attrs, a)
		return true
	})
	l.records++
	return nil
}

func (l *testLogger) WithAttrs(_ []slog.Attr) slog.Handler         { return l }
func (l *testLogger) WithGroup(_ string) slog.Handler              { return l }
func (l *testLogger) Enabled(_ context.Context, _ slog.Level) bool { return true }

type errorReader struct{}

func (errorReader) Read(_ []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}
func TestRequestLogger(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		body         io.Reader
		handler      http.HandlerFunc
		wantLevel    slog.Level
		wantMsg      string
		wantStatus   int
		wantErrorMsg string
	}{
		{
			name:       "success request",
			path:       "/api/test",
			handler:    func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
			wantLevel:  slog.LevelInfo,
			wantMsg:    "request success",
			wantStatus: http.StatusOK,
		},
		{
			name:         "client error request",
			path:         "/api/test",
			handler:      func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "bad request", http.StatusBadRequest) },
			wantLevel:    slog.LevelWarn,
			wantMsg:      "request failed",
			wantStatus:   http.StatusBadRequest,
			wantErrorMsg: "bad request\n",
		},
		{
			name: "server error request",
			path: "/api/test",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "server error", http.StatusInternalServerError)
			},
			wantLevel:    slog.LevelError,
			wantMsg:      "request error",
			wantStatus:   http.StatusInternalServerError,
			wantErrorMsg: "server error\n",
		},
		{
			name:    "skipped path",
			path:    "/metrics",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
		},
		{
			name:         "request body read error",
			path:         "/api/test",
			handler:      func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
			body:         &errorReader{},
			wantLevel:    slog.LevelError,
			wantMsg:      "failed to read request body",
			wantErrorMsg: "unexpected EOF",
		},
	}

	var skipPaths = []string{"metrics"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &testLogger{}
			handler := RequestLogger(slog.New(logger), skipPaths)(tt.handler)

			req := httptest.NewRequest(http.MethodPost, tt.path, tt.body)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantLevel, logger.level)
			assert.Equal(t, tt.wantMsg, logger.msg)

			var gotStatus int
			var gotErrorMsg string
			for _, attr := range logger.attrs {
				switch attr.Key {
				case "status":
					gotStatus = int(attr.Value.Int64())
				case "error":
					gotErrorMsg = attr.Value.String()
				}
			}

			assert.Equal(t, tt.wantStatus, gotStatus)
			assert.Equal(t, tt.wantErrorMsg, gotErrorMsg)
		})
	}
}
