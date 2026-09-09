package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/borisdvlpr/itero/internal/config"
	"github.com/borisdvlpr/itero/internal/response"
	chimw "github.com/go-chi/chi/v5/middleware"
)

type stubPinger struct{ err error }

func (s stubPinger) Ping(context.Context) error { return s.err }

// discardLogger formats records and throws them away, so the logging
// middleware still builds its attributes. slog.DiscardHandler would be
// shorter, but its Enabled reports false and RequestLogger returns early.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testServerConfig sets only what service() reads. RequestTimeout must be
// positive, or the timeout middleware is skipped and the router under test is
// not the router that ships.
func testServerConfig() config.ServerConfig {
	return config.ServerConfig{
		RequestTimeout:  5 * time.Second,
		MaxRequestBytes: 1 << 20,
	}
}

// serve runs one request through a fully wired router, so every test
// exercises the same middleware chain as production.
func serve(cfg config.ServerConfig, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	service(cfg, discardLogger(), Dependencies{DB: stubPinger{}}).ServeHTTP(rec, req)

	return rec
}

// oversizedRequest builds a POST whose declared length exceeds limit, the one
// request the router rejects before it has matched a route.
func oversizedRequest(path string, limit int64) *http.Request {
	body := strings.NewReader(strings.Repeat("x", int(limit)*4))
	return httptest.NewRequest(http.MethodPost, path, body)
}

type RouteTestCase struct {
	name       string
	path       string
	wantStatus int
}

func TestRoute(t *testing.T) {
	tests := []RouteTestCase{
		{
			name:       "liveness",
			path:       "/healthz",
			wantStatus: http.StatusOK,
		},
		{
			name:       "readiness",
			path:       "/readyz",
			wantStatus: http.StatusOK,
		},
		{
			name:       "unknown route",
			path:       "/does-not-exist",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(testServerConfig(), httptest.NewRequest(http.MethodGet, tc.path, nil))

			if rec.Code != tc.wantStatus {
				t.Errorf("GET %s = %d, want %d", tc.path, rec.Code, tc.wantStatus)
			}
		})
	}
}

func TestServicePropagatesRequestID(t *testing.T) {
	t.Run("generated when the client sends none", func(t *testing.T) {
		if got := requestIDFromProblem(t, ""); got == "" {
			t.Error("problem document carries no requestId")
		}
	})

	t.Run("client id is kept", func(t *testing.T) {
		if got := requestIDFromProblem(t, "ingress-42"); got != "ingress-42" {
			t.Errorf("requestId = %q, want the client's id", got)
		}
	})
}

// requestIDFromProblem sends an oversized request, tagged with clientID when
// one is given, and returns the requestId of the resulting problem document.
func requestIDFromProblem(t *testing.T, clientID string) string {
	t.Helper()

	cfg := testServerConfig()
	cfg.MaxRequestBytes = 16

	req := oversizedRequest("/healthz", cfg.MaxRequestBytes)
	if clientID != "" {
		req.Header.Set(chimw.RequestIDHeader, clientID)
	}

	rec := serve(cfg, req)

	var problem response.Problem
	if err := json.NewDecoder(rec.Body).Decode(&problem); err != nil {
		t.Fatalf("response is not a problem document: %v", err)
	}

	return problem.RequestID
}

type OversizedBodyTestCase struct {
	name            string
	path            string
	wantContentType string
	wantInBody      string
}

func TestOversizedBody(t *testing.T) {
	tests := []OversizedBodyTestCase{
		{
			name:            "Admin_path_renders_problem_details",
			path:            "/healthz",
			wantContentType: "application/problem+json",
			wantInBody:      `"title":"Request Entity Too Large"`,
		},
		{
			name:            "Ofrep_path_renders_Ofrep_shape",
			path:            "/ofrep/v1/evaluate/flags",
			wantContentType: "application/json",
			wantInBody:      `"errorCode":"GENERAL"`,
		},
	}

	cfg := testServerConfig()
	cfg.MaxRequestBytes = 16

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(cfg, oversizedRequest(tc.path, cfg.MaxRequestBytes))

			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
			}

			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, tc.wantContentType) {
				t.Errorf("Content-Type = %q, want %s", got, tc.wantContentType)
			}

			if !strings.Contains(rec.Body.String(), tc.wantInBody) {
				t.Errorf("body = %s, want it to contain %s", rec.Body.String(), tc.wantInBody)
			}
		})
	}
}
