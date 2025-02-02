package middleware

import "net/http"

type Middleware func(http.Handler) http.Handler

// Wrap wraps handler with middleware chain.
func Wrap(handler http.Handler, mw ...Middleware) http.Handler {
	for _, m := range mw {
		handler = m(handler)
	}

	return handler
}
