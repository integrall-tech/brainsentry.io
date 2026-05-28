package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/integraltech/brainsentry/internal/store"
)

// StoreMemoryHandler is the canonical-source CRUD surface backed by the
// pluggable store.MemoryStore interface. Existence proves the abstraction
// works end-to-end: cmd/server can wire either *store.PostgresStore (the
// production path) or *store.EmbeddedStore (the zero-config dev path) and
// every endpoint below keeps working without code changes.
//
// Why a separate handler instead of refactoring MemoryHandler?
// MemoryHandler holds a richer surface (versioning, embeddings, hybrid
// search, relationship management) that the small MemoryStore interface
// intentionally does not expose. Migrating MemoryHandler wholesale would
// either bloat MemoryStore or strand half its features. This handler is
// scoped to the operations the small interface covers, and routes mount
// it under /v1/store/memories so callers opt in.
type StoreMemoryHandler struct {
	store store.MemoryStore
}

// NewStoreMemoryHandler wires a handler around any MemoryStore impl.
func NewStoreMemoryHandler(s store.MemoryStore) *StoreMemoryHandler {
	return &StoreMemoryHandler{store: s}
}

// storeMemoryDTO is the JSON shape on the wire — keeps internal/store out
// of the public REST surface so the field set can evolve without breaking
// clients.
type storeMemoryDTO struct {
	ID         string   `json:"id,omitempty"`
	TenantID   string   `json:"tenantId,omitempty"`
	Content    string   `json:"content"`
	Summary    string   `json:"summary,omitempty"`
	Category   string   `json:"category,omitempty"`
	Importance string   `json:"importance,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	CreatedAt  string   `json:"createdAt,omitempty"`
	UpdatedAt  string   `json:"updatedAt,omitempty"`
}

func toDTO(m store.MemoryRecord) storeMemoryDTO {
	return storeMemoryDTO{
		ID: m.ID, TenantID: m.TenantID,
		Content: m.Content, Summary: m.Summary,
		Category: m.Category, Importance: m.Importance,
		Tags:      m.Tags,
		CreatedAt: m.CreatedAt.Format(rfc3339orEmpty(m.CreatedAt)),
		UpdatedAt: m.UpdatedAt.Format(rfc3339orEmpty(m.UpdatedAt)),
	}
}

func fromDTO(d storeMemoryDTO) store.MemoryRecord {
	return store.MemoryRecord{
		ID: d.ID, TenantID: d.TenantID,
		Content: d.Content, Summary: d.Summary,
		Category: d.Category, Importance: d.Importance,
		Tags: d.Tags,
	}
}

func rfc3339orEmpty(t interface {
	IsZero() bool
}) string {
	if t.IsZero() {
		return ""
	}
	return "2006-01-02T15:04:05.000Z07:00"
}

// Create handles POST /v1/store/memories.
func (h *StoreMemoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in storeMemoryDTO
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if in.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	created, err := h.store.Create(r.Context(), fromDTO(in))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toDTO(created))
}

// Get handles GET /v1/store/memories/{id}.
func (h *StoreMemoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	got, err := h.store.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "memory not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toDTO(got))
}

// List handles GET /v1/store/memories?limit=N. Defaults to 50.
func (h *StoreMemoryHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	rows, err := h.store.List(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]storeMemoryDTO, len(rows))
	for i, r := range rows {
		out[i] = toDTO(r)
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out, "total": len(out)})
}

// Search handles GET /v1/store/memories/search?q=...&limit=N.
func (h *StoreMemoryHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "q is required")
		return
	}
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	rows, err := h.store.Search(r.Context(), q, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]storeMemoryDTO, len(rows))
	for i, r := range rows {
		out[i] = toDTO(r)
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out, "total": len(out)})
}

// Delete handles DELETE /v1/store/memories/{id}. Idempotent — missing IDs
// still return 204 to keep the contract symmetric with the embedded store.
func (h *StoreMemoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
