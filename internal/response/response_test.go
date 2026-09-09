package response

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
)

const testRequestID = "req-0123456789"

func TestJSONWritesBodyAndHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	JSON(rec, newRequest(ProblemJSON, "/healthz"), http.StatusCreated, map[string]string{"status": "ok"})

	assertResponse(t, rec, http.StatusCreated, contentTypeJSON, `{"status":"ok"}`)
}

type ProblemDetailsTestCase struct {
	name       string
	err        APIError
	wantStatus int
	wantBody   string
}

func TestProblemDetailsSchemas(t *testing.T) {
	tests := []ProblemDetailsTestCase{
		{
			name:       "Details_instance_and_request_id_are_present",
			err:        APIError{Status: http.StatusBadRequest, Detail: "name is required"},
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"title":"Bad Request","status":400,"detail":"name is required","instance":"/api/v1/flags","requestId":"req-0123456789"}`,
		},
		{
			name: "Type_is_rendered_when_given",
			err: APIError{
				Status: http.StatusConflict,
				Detail: "a flag with that key already exists",
				Type:   "https://itero.dev/problems/duplicate-flag",
			},
			wantStatus: http.StatusConflict,
			wantBody:   `{"type":"https://itero.dev/problems/duplicate-flag","title":"Conflict","status":409,"detail":"a flag with that key already exists","instance":"/api/v1/flags","requestId":"req-0123456789"}`,
		},
		{
			name:       "Missing_status_defaults_to_500",
			err:        APIError{},
			wantStatus: http.StatusInternalServerError,
			wantBody:   `{"title":"Internal Server Error","status":500,"instance":"/api/v1/flags","requestId":"req-0123456789"}`,
		},
		{
			name: "Ofrep_only_fields_are_dropped",
			err: APIError{
				Status: http.StatusNotFound,
				Detail: "no such flag",
				Code:   CodeFlagNotFound,
				Key:    "dark-mode",
			},
			wantStatus: http.StatusNotFound,
			wantBody:   `{"title":"Not Found","status":404,"detail":"no such flag","instance":"/api/v1/flags","requestId":"req-0123456789"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteError(rec, newRequest(ProblemJSON, "/api/v1/flags"), tc.err)

			assertResponse(t, rec, tc.wantStatus, contentTypeProblem, tc.wantBody)
		})
	}
}

// A request that never passed through chi's RequestID still gets a valid
// document; the id is simply left out. Going through Error rather than
// WriteError also covers the convenience wrapper.
func TestProblemOmitsRequestIDWhenAbsent(t *testing.T) {
	rec := httptest.NewRecorder()
	Error(rec, httptest.NewRequest(http.MethodGet, "/api/v1/flags", nil), http.StatusBadRequest, "name is required")

	assertResponse(t, rec, http.StatusBadRequest, contentTypeProblem,
		`{"title":"Bad Request","status":400,"detail":"name is required","instance":"/api/v1/flags"}`)
}

type errorFromTestCase struct {
	name       string
	err        error
	wantStatus int
	wantBody   string
}

func TestErrorFrom(t *testing.T) {
	tests := []errorFromTestCase{
		{
			name:       "MaxBytesError_becomes_413_when_body_exceeds_limit",
			err:        &http.MaxBytesError{Limit: 1024},
			wantStatus: http.StatusRequestEntityTooLarge,
			wantBody:   `{"title":"Request Entity Too Large","status":413,"detail":"request body exceeds 1024 bytes","instance":"/api/v1/flags","requestId":"req-0123456789"}`,
		},
		{
			name:       "MaxBytesError_wrapped_is_recognized",
			err:        fmt.Errorf("decode request body: %w", &http.MaxBytesError{Limit: 512}),
			wantStatus: http.StatusRequestEntityTooLarge,
			wantBody:   `{"title":"Request Entity Too Large","status":413,"detail":"request body exceeds 512 bytes","instance":"/api/v1/flags","requestId":"req-0123456789"}`,
		},
		{
			name:       "Unrecognized_errors_become_500",
			err:        errors.New(`pq: relation "flags" does not exist`),
			wantStatus: http.StatusInternalServerError,
			wantBody:   `{"title":"Internal Server Error","status":500,"detail":"an unexpected error occurred","instance":"/api/v1/flags","requestId":"req-0123456789"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			quietLogs(t)

			rec := httptest.NewRecorder()
			ErrorFrom(rec, newRequest(ProblemJSON, "/api/v1/flags"), tc.err)

			assertResponse(t, rec, tc.wantStatus, contentTypeProblem, tc.wantBody)
		})
	}
}

type FallbackTestCase struct {
	name            string
	dialect         Dialect
	wantContentType string
	wantBody        string
}

func TestFallback(t *testing.T) {
	tests := []FallbackTestCase{
		{
			name:            "Problem_details_fallback",
			dialect:         ProblemJSON,
			wantContentType: contentTypeProblem,
			wantBody:        problemFallback,
		},
		{
			name:            "Ofrep_fallback",
			dialect:         OFREP,
			wantContentType: contentTypeJSON,
			wantBody:        ofrepFallback,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			quietLogs(t)

			rec := httptest.NewRecorder()

			JSON(rec, newRequest(tc.dialect, "/api/v1/flags"), http.StatusOK, unmarshalable{})

			assertResponse(t, rec, http.StatusInternalServerError, tc.wantContentType, tc.wantBody)

			var parsed map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
				t.Errorf("fallback body is not valid JSON: %v", err)
			}
		})
	}
}

// assertResponse checks status, Content-Type, the nosniff header every writer
// sets, and the exact body. json.Encoder terminates the body with a newline,
// which is trimmed so cases can be written as one-line literals.
func assertResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantContentType, wantBody string) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Errorf("status = %d, want %d", rec.Code, wantStatus)
	}

	if got := rec.Header().Get("Content-Type"); got != wantContentType {
		t.Errorf("Content-Type = %q, want %q", got, wantContentType)
	}

	// Without nosniff a browser may re-interpret a JSON body as HTML.
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}

	if got := strings.TrimSpace(rec.Body.String()); got != wantBody {
		t.Errorf("body = %s\nwant = %s", got, wantBody)
	}
}

// quietLogs silences the package-level logger for the duration of a test.
// Several paths here log deliberately — an encode failure, an unrecognised
// error — and their records would otherwise land in the test output as if
// something had gone wrong. It swaps a global, so tests that call it must not
// use t.Parallel.
func quietLogs(t *testing.T) {
	t.Helper()

	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(io.Discard, nil)))

	t.Cleanup(func() { slog.SetDefault(previous) })
}

// newRequest builds a request already carrying a dialect and a request id,
// bypassing the middleware so the writers can be tested in isolation.
func newRequest(dialect Dialect, path string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, nil)

	ctx := context.WithValue(r.Context(), middleware.RequestIDKey, testRequestID)
	if dialect != ProblemJSON {
		ctx = context.WithValue(ctx, dialectKey{}, dialect)
	}

	return r.WithContext(ctx)
}

// unmarshalable makes json.Encoder fail: a channel has no JSON representation,
// which is the only practical way to reach the fallback path.
type unmarshalable struct {
	Ch chan int `json:"ch"`
}
