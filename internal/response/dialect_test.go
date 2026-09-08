package response

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type DialectTestCase struct {
	name     string
	prefixes []string
	path     string
	want     Dialect
}

func TestDialect(t *testing.T) {
	tests := []DialectTestCase{
		{
			name:     "Path_under_the_prefix_speaks_OFREP",
			prefixes: []string{"/ofrep"},
			path:     "/ofrep/v1/evaluate/flags/dark-mode",
			want:     OFREP,
		},
		{
			name:     "The_prefix_speaks_OFREP",
			prefixes: []string{"/ofrep"},
			path:     "/ofrep",
			want:     OFREP,
		},
		{
			name:     "Admin_path_keeps_problem_details",
			prefixes: []string{"/ofrep"},
			path:     "/api/v1/flags",
			want:     ProblemJSON,
		},
		{
			name:     "Probe_path_keeps_problem_details",
			prefixes: []string{"/ofrep"},
			path:     "/readyz",
			want:     ProblemJSON,
		},
		{
			name:     "Any_of_several_prefixes_matches",
			prefixes: []string{"/ofrep", "/evaluate"},
			path:     "/evaluate/flags",
			want:     OFREP,
		},
		{
			name:     "No_prefix_keeps_problem_details",
			prefixes: nil,
			path:     "/ofrep/v1/evaluate/flags",
			want:     ProblemJSON,
		},
		{
			name:     "Defaults_to_problem_details",
			prefixes: []string{"/anything"},
			path:     "/anything",
			want:     ProblemJSON,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got Dialect

			next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = dialectFrom(r.Context())
			})

			rec := httptest.NewRecorder()
			Dialects(tc.prefixes...)(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))

			if got != tc.want {
				t.Errorf("dialect = %v, want %v", got, tc.want)
			}
		})
	}
}
