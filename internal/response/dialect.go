package response

import (
	"context"
	"net/http"
	"strings"
)

// Dialect selects the wire format used for error responses. itero speaks two:
// RFC 9457 problem details for the admin API, and OFREP's own error shapes
// under the evaluation namespace. The dialect is a property of the request
// rather than of the call site, so middleware that cannot know which route will
// match — the body limit runs before chi has matched anything — still emits the
// correct shape.
type Dialect uint8

const (
	// ProblemJSON is the zero value, so any request not explicitly marked
	// otherwise falls back to RFC 9457.
	ProblemJSON Dialect = iota
	OFREP
)

type dialectKey struct{}

// Dialects marks requests whose path begins with any of prefixes as speaking
// OFREP. It must be registered ahead of every middleware that can write an
// error response, since chi runs root-level middleware before route matching
// and a sub-router's own middleware would therefore be too late.
func Dialects(prefixes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, p := range prefixes {
				if strings.HasPrefix(r.URL.Path, p) {
					r = r.WithContext(context.WithValue(r.Context(), dialectKey{}, OFREP))
					break
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func dialectFrom(ctx context.Context) Dialect {
	d, _ := ctx.Value(dialectKey{}).(Dialect)
	return d
}
