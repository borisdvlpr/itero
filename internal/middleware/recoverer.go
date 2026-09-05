package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/borisdvlpr/itero/internal/response"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// Recoverer converts a handler panic into a logged event and a 500 rendered in
// the request's dialect, unlike chi's own Recoverer, which only produces
// structured output when paired with chi's RequestLogger — itero uses its
// own — and otherwise prints a coloured stack trace to stdout that won't
// parse as JSON.
func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rvr := recover()
				if rvr == nil {
					return
				}

				// net/http uses this sentinel to abort a connection deliberately; it must keep propagating
				if err, ok := rvr.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(rvr)
				}

				logger.LogAttrs(r.Context(), slog.LevelError, "panic recovered",
					slog.Any("panic", rvr),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("request_id", chimw.GetReqID(r.Context())),
					slog.String("stack", string(debug.Stack())),
				)

				response.Error(w, r, http.StatusInternalServerError, "an unexpected error occurred")
			}()

			next.ServeHTTP(w, r)
		})
	}
}
