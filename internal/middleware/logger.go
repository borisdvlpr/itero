package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// RequestLogger emits one structured record per request, writing real slog
// attributes rather than chi's preformatted text line so logs stay queryable by
// status, route and latency. Paths listed in quietPaths are demoted to debug
// when they succeed, keeping frequent Kubernetes probes out of the log volume,
// but keep their warn or error level on failure.
func RequestLogger(logger *slog.Logger, quietPaths ...string) func(http.Handler) http.Handler {
	quiet := make(map[string]struct{}, len(quietPaths))
	for _, p := range quietPaths {
		quiet[p] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			defer func() {
				status := ww.Status()
				if status == 0 {
					// handler returned without writing anything; net/http will have sent 200
					status = http.StatusOK
				}

				level := levelForStatus(status)
				if _, ok := quiet[r.URL.Path]; ok && level == slog.LevelInfo {
					level = slog.LevelDebug
				}

				// checked before the attributes are built so a disabled level costs nothing beyond the lookup
				ctx := r.Context()
				if !logger.Enabled(ctx, level) {
					return
				}

				elapsed := time.Since(start)

				attrs := []slog.Attr{
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", status),
					slog.Int("bytes", ww.BytesWritten()),
					slog.Float64("duration_ms", float64(elapsed.Microseconds())/1000),
					slog.String("remote_addr", r.RemoteAddr),
				}

				if pattern := routePattern(ctx); pattern != "" {
					attrs = append(attrs, slog.String("route", pattern))
				}

				if q := r.URL.RawQuery; q != "" {
					attrs = append(attrs, slog.String("query", q))
				}

				if id := chimw.GetReqID(ctx); id != "" {
					attrs = append(attrs, slog.String("request_id", id))
				}

				logger.LogAttrs(ctx, level, "http request", attrs...)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

// routePattern returns the matched route template, for example
// "/api/v1/flags/{key}", which unlike path has bounded cardinality and so can be
// aggregated per endpoint rather than per flag key. It is empty when no route
// matched or when this middleware is mounted outside a chi router.
func routePattern(ctx context.Context) string {
	rctx := chi.RouteContext(ctx)
	if rctx == nil {
		return ""
	}

	return rctx.RoutePattern()
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
