package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/borisdvlpr/itero/internal/response"
	"github.com/go-chi/chi/v5"
)

// Pingable is the only thing the health handler needs from the database, which
// keeps it testable without a live Postgres.
type Pingable interface {
	Ping(ctx context.Context) error
}

type HealthHandler struct {
	db      Pingable
	timeout time.Duration
}

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database,omitempty"`
}

func NewHealthHandler(db Pingable, timeout time.Duration) *HealthHandler {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	return &HealthHandler{
		db:      db,
		timeout: timeout,
	}
}

func (h *HealthHandler) Routes(r chi.Router) {
	r.Get("/healthz", h.liveness)
	r.Get("/readyz", h.readiness)
}

// liveness answers "is this process wedged". It deliberately does not touch
// the database: a failing liveness probe restarts the pod, and restarting an
// application because Postgres is down helps nobody.
func (h *HealthHandler) liveness(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, r, http.StatusOK, healthResponse{Status: "ok"})
}

// readiness answers "should this pod receive traffic", which does depend on
// the database being reachable.
func (h *HealthHandler) readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		response.JSON(w, r, http.StatusServiceUnavailable, healthResponse{
			Status:   "unavailable",
			Database: "unreachable",
		})
		return
	}

	response.JSON(w, r, http.StatusOK, healthResponse{
		Status:   "ready",
		Database: "ok",
	})
}
