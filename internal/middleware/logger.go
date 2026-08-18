package middleware

import (
	"log/slog"
	"net/http"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// RequestLogger emits one structured record per request. Unlike chi's
// RequestLogger it writes real slog attributes rather than a preformatted text
// line, so logs stay queryable by status, path and latency.
//
// Paths listed in skipPaths are served without logging; Kubernetes probes hit
// the health endpoints every few seconds per pod and would otherwise dominate
// the log volume.
func RequestLogger(logger *slog.Logger, skipPaths ...string) func(http.Handler) http.Handler {
	skip := make(map[string]struct{}, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, skipped := skip[r.URL.Path]; skipped {
				next.ServeHTTP(w, r)
				return
			}

			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			defer func() {
				status := ww.Status()
				if status == 0 {
					// Handler returned without writing anything; net/http will have sent 200
					status = http.StatusOK
				}

				attrs := []any{
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", status),
					slog.Int("bytes", ww.BytesWritten()),
					slog.Duration("duration", time.Since(start)),
					slog.String("remote_addr", r.RemoteAddr),
					slog.String("request_id", chimw.GetReqID(r.Context())),
				}

				if q := r.URL.RawQuery; q != "" {
					attrs = append(attrs, slog.String("query", q))
				}

				logger.Log(r.Context(), levelForStatus(status), "http request", attrs...)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

func levelForStatus(status int) slog.Level {
	switch {
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status >= http.StatusBadRequest:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}
