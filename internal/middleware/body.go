// Package middleware holds itero's own HTTP middleware. chi's middleware
// package is imported here as chimw to keep the two distinguishable.
package middleware

import (
	"fmt"
	"net/http"

	"github.com/borisdvlpr/itero/internal/response"
)

// MaxBodyBytes limits request bodies to limit bytes. A declared oversize is
// rejected here; a chunked or under-declared body instead fails when the
// handler reads it, with *http.MaxBytesError, which response.ErrorFrom maps to
// the same 413.
func MaxBodyBytes(limit int64) func(http.Handler) http.Handler {
	detail := fmt.Sprintf("request body exceeds %d bytes", limit)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > limit {
				response.Error(w, r, http.StatusRequestEntityTooLarge, detail)
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}
