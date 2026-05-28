package handler

import (
	"net/http"

	"github.com/integraltech/brainsentry/internal/eval"
)

// EvalHandler exposes the in-memory eval candidate buffer over HTTP. The
// buffer is shared with the in-process retrieval path: when the operator
// has set BRAINSENTRY_EVAL_CAPTURE=1, every recall feeds the store; this
// handler exports it as NDJSON.
type EvalHandler struct {
	Store *eval.Store
}

// NewEvalHandler wires a handler around a Store.
func NewEvalHandler(s *eval.Store) *EvalHandler {
	return &EvalHandler{Store: s}
}

// Export handles GET /v1/eval/candidates.ndjson. Streams the entire buffer
// as application/x-ndjson. Callers (CLI / CI) can pipe straight to a file.
func (h *EvalHandler) Export(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	if _, err := h.Store.Export(w); err != nil {
		// best-effort: response headers already flushed; nothing actionable
		// beyond letting the connection close.
		_ = err
	}
}

// Stats handles GET /v1/eval/candidates/stats. Tiny JSON for admin UIs.
func (h *EvalHandler) Stats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"count":   h.Store.Len(),
		"capture": eval.CaptureEnabled(),
	})
}

// Reset handles POST /v1/eval/candidates/reset. Empties the buffer (used
// after a successful export).
func (h *EvalHandler) Reset(w http.ResponseWriter, _ *http.Request) {
	h.Store.Reset()
	writeJSON(w, http.StatusOK, map[string]any{"reset": true})
}
