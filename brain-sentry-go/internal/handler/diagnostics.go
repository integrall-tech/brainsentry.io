package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/integraltech/brainsentry/internal/diagnostics"
)

// DiagnosticsHandler exposes the doctor report over HTTP. The same Doctor
// instance is shared with the CLI command so behavior cannot drift.
type DiagnosticsHandler struct {
	Doctor *diagnostics.Doctor
}

// NewDiagnosticsHandler wires a handler around a configured Doctor.
func NewDiagnosticsHandler(d *diagnostics.Doctor) *DiagnosticsHandler {
	return &DiagnosticsHandler{Doctor: d}
}

// Get handles GET /v1/diagnostics. Returns 200 even when checks fail; the
// JSON body's `status` field tells you the aggregate. CI / admin UIs key off
// the body, not the HTTP status code — so a 200 with `status:"fail"` is
// intentional and matches the CLI's structured output.
func (h *DiagnosticsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.Doctor == nil {
		writeError(w, http.StatusServiceUnavailable, "doctor not configured")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	rep := h.Doctor.Run(ctx)
	writeJSON(w, http.StatusOK, rep)
}
