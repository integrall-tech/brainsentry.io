package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/integraltech/brainsentry/internal/models"
)

// ModelsHandler exposes the tier-routing snapshot and the doctor probe over
// HTTP. Same engine as the CLI so they cannot drift.
type ModelsHandler struct {
	Config models.Config
	Prober models.Prober
}

// NewModelsHandler wires a handler given the resolved tier config and a
// prober (the prober may be nil; in that case Doctor returns a clear
// "no prober wired" failure per tier so the operator knows to configure a
// provider).
func NewModelsHandler(cfg models.Config, prober models.Prober) *ModelsHandler {
	return &ModelsHandler{Config: cfg, Prober: prober}
}

// SnapshotResponse is the read-only routing dashboard payload.
type SnapshotResponse struct {
	Snapshot []models.ResolveResult `json:"snapshot"`
}

// List handles GET /v1/models. Returns the current resolution per tier
// (model id + which rule matched). Public-safe by design — exposes
// configuration intent, not secrets.
func (h *ModelsHandler) List(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, SnapshotResponse{Snapshot: models.Snapshot(h.Config)})
}

// Doctor handles GET /v1/models/doctor. Runs a 1-token probe against every
// tier, classifies failures into actionable categories. Returns 200 always
// — the body's `ok` field is the contract for CI gating.
func (h *ModelsHandler) Doctor(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	rep := models.RunDoctor(ctx, h.Config, h.Prober, 8*time.Second)
	writeJSON(w, http.StatusOK, rep)
}
