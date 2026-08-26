package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

type StubPinger struct {
	err   error
	delay time.Duration
}

func (s StubPinger) Ping(ctx context.Context) error {
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return s.err
}

func newTestRouter(db Pingable, timeout time.Duration) http.Handler {
	r := chi.NewRouter()
	NewHealthHandler(db, timeout).Routes(r)

	return r
}

func TestLiveness_IgnoresDatabase(t *testing.T) {
	// A wedged database must not restart the pod, so /healthz has to stay green even when Ping fails.
	router := newTestRouter(StubPinger{err: errors.New("connection refused")}, time.Second)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want JSON", got)
	}
}

type ReadinessTestCase struct {
	id              string
	db              Pingable
	timeout         time.Duration
	wantStatus      int
	wantStatusField string
}

func TestReadiness(t *testing.T) {
	tests := []ReadinessTestCase{
		{
			id:              "DatabaseReachable",
			db:              StubPinger{},
			timeout:         time.Second,
			wantStatus:      http.StatusOK,
			wantStatusField: "ready",
		},
		{
			id:              "DatabaseUnreachable",
			db:              StubPinger{err: errors.New("connection refused")},
			timeout:         time.Second,
			wantStatus:      http.StatusServiceUnavailable,
			wantStatusField: "unavailable",
		},
		{
			id:              "DatabaseTooSlow",
			db:              StubPinger{delay: 50 * time.Millisecond},
			timeout:         time.Millisecond,
			wantStatus:      http.StatusServiceUnavailable,
			wantStatusField: "unavailable",
		},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			router := newTestRouter(tc.db, tc.timeout)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			var body healthResponse
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("response is not valid JSON: %v", err)
			}

			if body.Status != tc.wantStatusField {
				t.Errorf("status field = %q, want %q", body.Status, tc.wantStatusField)
			}
		})
	}
}
