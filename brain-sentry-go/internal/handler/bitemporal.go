package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
)

// BiTemporalHandler exposes "as of" time-travel queries over Memory.
type BiTemporalHandler struct {
	repo *postgres.MemoryRepository
}

// NewBiTemporalHandler wires the repository.
func NewBiTemporalHandler(repo *postgres.MemoryRepository) *BiTemporalHandler {
	return &BiTemporalHandler{repo: repo}
}

// parseTemporalQuery extracts the shared (timestamp, limit) inputs of the
// bi-temporal endpoints from a query string. Pure + side-effect free so
// the parsing/validation rules are unit-testable without a DB or HTTP
// server. `tsParam` is the name of the required RFC3339 timestamp param
// ("at" for as-of, "since" for changed-since). limit defaults to 100 and
// ignores non-positive / unparseable values.
func parseTemporalQuery(q url.Values, tsParam string) (time.Time, int, error) {
	v := q.Get(tsParam)
	if v == "" {
		return time.Time{}, 0, fmt.Errorf("%s (RFC3339 timestamp) is required", tsParam)
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid RFC3339 timestamp")
	}
	limit := 100
	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	return t, limit, nil
}

// AsOf handles GET /v1/memories/as-of?at=<RFC3339>&limit=N
func (h *BiTemporalHandler) AsOf(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "memory repository not available")
		return
	}
	t, limit, err := parseTemporalQuery(r.URL.Query(), "at")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	list, err := h.repo.FindAsOf(r.Context(), t, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	list = nonNilMemories(list)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"count":    len(list),
		"asOf":     t.UTC().Format(time.RFC3339),
		"memories": list,
	})
}

// ChangedSince handles GET /v1/memories/changed-since?since=<RFC3339>&limit=N
// — the incremental-sync complement of AsOf. Returns memories created or
// updated at/after the given instant, newest change first.
func (h *BiTemporalHandler) ChangedSince(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, "memory repository not available")
		return
	}
	t, limit, err := parseTemporalQuery(r.URL.Query(), "since")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	list, err := h.repo.FindChangedSince(r.Context(), t, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	list = nonNilMemories(list)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"count":    len(list),
		"since":    t.UTC().Format(time.RFC3339),
		"memories": list,
	})
}

// nonNilMemories guarantees an empty result serialises as a JSON [] not
// null — a stricter, friendlier API contract for clients that call
// .map/.some on the array without a null guard. (scanMemories returns a
// nil slice when there are no rows.)
func nonNilMemories(list []domain.Memory) []domain.Memory {
	if list == nil {
		return []domain.Memory{}
	}
	return list
}
