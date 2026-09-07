package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serveWithLogger(t *testing.T, level slog.Level, path string, status int, quiet ...string) string {
	t.Helper()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: level}))

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	})

	rec := httptest.NewRecorder()
	RequestLogger(logger, quiet...)(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	return logs.String()
}

func TestRequestLoggerEmitsStructuredFields(t *testing.T) {
	out := serveWithLogger(t, slog.LevelDebug, "/flags?env=prod", http.StatusOK)

	fields := []string{
		`"method":"GET"`,
		`"path":"/flags"`,
		`"status":200`,
		`"query":"env=prod"`,
		`"duration_ms"`,
	}

	for _, field := range fields {
		if !strings.Contains(out, field) {
			t.Errorf("log output missing %s: %s", field, out)
		}
	}
}

func TestRequestLoggerReportsDurationInMilliseconds(t *testing.T) {
	out := serveWithLogger(t, slog.LevelDebug, "/flags", http.StatusOK)

	if strings.Contains(out, `"duration":`) {
		t.Errorf("log still carries the nanosecond duration field: %s", out)
	}
}

type RequestLoggerLevelsTestCase struct {
	name      string
	status    int
	wantLevel string
}

func TestRequestLoggerLevels(t *testing.T) {
	tests := []RequestLoggerLevelsTestCase{
		{
			name:      "Success_is_informational",
			status:    http.StatusOK,
			wantLevel: `"level":"INFO"`,
		},
		{
			name:      "Client_error_is_a_warning",
			status:    http.StatusNotFound,
			wantLevel: `"level":"WARN"`,
		},
		{
			name:      "Server_error_is_an_error",
			status:    http.StatusInternalServerError,
			wantLevel: `"level":"ERROR"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := serveWithLogger(t, slog.LevelDebug, "/flags", tc.status)

			if !strings.Contains(out, tc.wantLevel) {
				t.Errorf("status %d logged at wrong level: %s", tc.status, out)
			}
		})
	}
}

type RequestLoggerQuietPathsTestCase struct {
	name      string
	level     slog.Level
	status    int
	wantLevel string
}

func TestRequestLoggerQuietPaths(t *testing.T) {
	tests := []RequestLoggerQuietPathsTestCase{
		{
			name:      "Successful_probe_is_silent_at_info",
			level:     slog.LevelInfo,
			status:    http.StatusOK,
			wantLevel: "",
		},
		{
			name:      "Successful_probe_is_demoted_to_debug",
			level:     slog.LevelDebug,
			status:    http.StatusOK,
			wantLevel: `"level":"DEBUG"`,
		},
		{
			name:      "Failing_probe_stays_visible_at_info",
			level:     slog.LevelInfo,
			status:    http.StatusServiceUnavailable,
			wantLevel: `"level":"ERROR"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := serveWithLogger(t, tc.level, "/readyz", tc.status, "/healthz", "/readyz")

			if tc.wantLevel == "" {
				if out != "" {
					t.Errorf("expected no log record, got: %s", out)
				}

				return
			}

			if !strings.Contains(out, tc.wantLevel) {
				t.Errorf("log record missing %s: %s", tc.wantLevel, out)
			}
		})
	}
}
