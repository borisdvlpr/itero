package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecovererReturnsProblemJSON(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})

	rec := httptest.NewRecorder()
	Recoverer(logger)(panicking).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/flags", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/problem+json") {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	if strings.Contains(rec.Body.String(), "boom") {
		t.Error("response body leaked the panic value")
	}

	if !strings.Contains(logs.String(), "panic recovered") {
		t.Error("panic was not logged")
	}

	if !strings.Contains(logs.String(), "stack") {
		t.Error("log record is missing the stack trace")
	}
}

func TestRecovererPassesThroughNormalResponses(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	rec := httptest.NewRecorder()
	Recoverer(logger)(ok).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/flags", nil))

	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
}
