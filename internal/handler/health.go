package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type Pingable interface {
	Ping(ctx context.Context) error
}

type HealthHandler struct {
	db Pingable
}

func NewHealthHandler(db Pingable) *HealthHandler {
	return &HealthHandler{
		db: db,
	}
}

func (h *HealthHandler) Routes(r chi.Router) {
	r.Get("/healthz", h.liveness)
	r.Get("/readyz", h.readiness)
}

func (h *HealthHandler) liveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (h *HealthHandler) readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Database unreachable"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ready"))
}
