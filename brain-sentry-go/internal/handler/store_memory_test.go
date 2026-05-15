package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/integraltech/brainsentry/internal/store"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// router builds a chi.Router that mirrors the production wiring so chi's
// URL params resolve correctly inside handler tests.
func storeRouter(h *StoreMemoryHandler) chi.Router {
	r := chi.NewRouter()
	r.Route("/v1/store/memories", func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/search", h.Search)
		r.Get("/{id}", h.Get)
		r.Delete("/{id}", h.Delete)
	})
	return r
}

func newTestEmbedded(t *testing.T) store.MemoryStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "brain.db.json")
	s, err := store.OpenEmbedded(path)
	if err != nil {
		t.Fatalf("open embedded: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func doJSON(t *testing.T, r chi.Router, method, url, body, tenantID string) *httptest.ResponseRecorder {
	t.Helper()
	var buf *bytes.Buffer
	if body != "" {
		buf = bytes.NewBufferString(body)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, url, buf)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if tenantID != "" {
		req = req.WithContext(tenant.WithTenant(req.Context(), tenantID))
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

// --- End-to-end: same handler against EmbeddedStore ---

func TestStoreMemory_CreateThenGet(t *testing.T) {
	s := newTestEmbedded(t)
	h := NewStoreMemoryHandler(s)
	r := storeRouter(h)

	rr := doJSON(t, r, http.MethodPost, "/v1/store/memories",
		`{"content":"hello","summary":"sum","category":"INSIGHT"}`, "t1")
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: expected 201; got %d body=%s", rr.Code, rr.Body.String())
	}
	var created storeMemoryDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ID == "" || created.Content != "hello" {
		t.Errorf("expected stamped id + echoed content; got %+v", created)
	}

	rr = doJSON(t, r, http.MethodGet, "/v1/store/memories/"+created.ID, "", "t1")
	if rr.Code != http.StatusOK {
		t.Fatalf("get: expected 200; got %d body=%s", rr.Code, rr.Body.String())
	}
	var got storeMemoryDTO
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if got.Summary != "sum" {
		t.Errorf("expected summary roundtripped; got %+v", got)
	}
}

func TestStoreMemory_CreateRejectsEmptyContent(t *testing.T) {
	r := storeRouter(NewStoreMemoryHandler(newTestEmbedded(t)))
	rr := doJSON(t, r, http.MethodPost, "/v1/store/memories", `{"summary":"x"}`, "t1")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400; got %d", rr.Code)
	}
}

func TestStoreMemory_CreateRejectsBadJSON(t *testing.T) {
	r := storeRouter(NewStoreMemoryHandler(newTestEmbedded(t)))
	rr := doJSON(t, r, http.MethodPost, "/v1/store/memories", `not json`, "t1")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400; got %d", rr.Code)
	}
}

func TestStoreMemory_GetMissing404(t *testing.T) {
	r := storeRouter(NewStoreMemoryHandler(newTestEmbedded(t)))
	rr := doJSON(t, r, http.MethodGet, "/v1/store/memories/no-such", "", "t1")
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404; got %d", rr.Code)
	}
}

func TestStoreMemory_ListPaginatesByLimit(t *testing.T) {
	s := newTestEmbedded(t)
	r := storeRouter(NewStoreMemoryHandler(s))
	for i := 0; i < 5; i++ {
		_ = doJSON(t, r, http.MethodPost, "/v1/store/memories",
			`{"content":"row"}`, "t1")
	}
	rr := doJSON(t, r, http.MethodGet, "/v1/store/memories?limit=3", "", "t1")
	if rr.Code != http.StatusOK {
		t.Fatalf("list: expected 200; got %d", rr.Code)
	}
	var page struct {
		Results []storeMemoryDTO `json:"results"`
		Total   int              `json:"total"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &page)
	if page.Total != 3 {
		t.Errorf("expected limit=3 honored; got total=%d", page.Total)
	}
}

func TestStoreMemory_SearchRanksContent(t *testing.T) {
	s := newTestEmbedded(t)
	r := storeRouter(NewStoreMemoryHandler(s))
	for _, body := range []string{
		`{"content":"postgres backup procedure"}`,
		`{"content":"javascript event loop"}`,
		`{"content":"postgres tuning notes"}`,
	} {
		_ = doJSON(t, r, http.MethodPost, "/v1/store/memories", body, "t1")
	}
	rr := doJSON(t, r, http.MethodGet, "/v1/store/memories/search?q=postgres&limit=5", "", "t1")
	if rr.Code != http.StatusOK {
		t.Fatalf("search: expected 200; got %d body=%s", rr.Code, rr.Body.String())
	}
	var page struct {
		Results []storeMemoryDTO `json:"results"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &page)
	if len(page.Results) != 2 {
		t.Errorf("expected 2 hits for 'postgres'; got %d", len(page.Results))
	}
}

func TestStoreMemory_SearchRequiresQ(t *testing.T) {
	r := storeRouter(NewStoreMemoryHandler(newTestEmbedded(t)))
	rr := doJSON(t, r, http.MethodGet, "/v1/store/memories/search", "", "t1")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 without q; got %d", rr.Code)
	}
}

func TestStoreMemory_DeleteThenGet404(t *testing.T) {
	s := newTestEmbedded(t)
	r := storeRouter(NewStoreMemoryHandler(s))
	rr := doJSON(t, r, http.MethodPost, "/v1/store/memories", `{"content":"x"}`, "t1")
	var created storeMemoryDTO
	_ = json.Unmarshal(rr.Body.Bytes(), &created)

	rr = doJSON(t, r, http.MethodDelete, "/v1/store/memories/"+created.ID, "", "t1")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204; got %d", rr.Code)
	}
	rr = doJSON(t, r, http.MethodGet, "/v1/store/memories/"+created.ID, "", "t1")
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete; got %d", rr.Code)
	}
}

func TestStoreMemory_DeleteIdempotent(t *testing.T) {
	r := storeRouter(NewStoreMemoryHandler(newTestEmbedded(t)))
	rr := doJSON(t, r, http.MethodDelete, "/v1/store/memories/no-such", "", "t1")
	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204 for idempotent delete; got %d", rr.Code)
	}
}

func TestStoreMemory_TenantScopedAcrossEndpoints(t *testing.T) {
	s := newTestEmbedded(t)
	r := storeRouter(NewStoreMemoryHandler(s))
	rr := doJSON(t, r, http.MethodPost, "/v1/store/memories", `{"content":"private"}`, "t1")
	var created storeMemoryDTO
	_ = json.Unmarshal(rr.Body.Bytes(), &created)

	// Different tenant cannot read t1's memory
	rr = doJSON(t, r, http.MethodGet, "/v1/store/memories/"+created.ID, "", "t2")
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for cross-tenant read; got %d body=%s", rr.Code, rr.Body.String())
	}
}

// --- Failure surface: backend returning errors ---

type errStore struct{ err error }

func (e *errStore) Create(_ context.Context, _ store.MemoryRecord) (store.MemoryRecord, error) {
	return store.MemoryRecord{}, e.err
}
func (e *errStore) Get(_ context.Context, _ string) (store.MemoryRecord, error) {
	return store.MemoryRecord{}, e.err
}
func (e *errStore) List(_ context.Context, _ int) ([]store.MemoryRecord, error) {
	return nil, e.err
}
func (e *errStore) Search(_ context.Context, _ string, _ int) ([]store.MemoryRecord, error) {
	return nil, e.err
}
func (e *errStore) Delete(_ context.Context, _ string) error { return e.err }
func (e *errStore) Close() error                              { return nil }

func TestStoreMemory_BackendErrorReturns500(t *testing.T) {
	r := storeRouter(NewStoreMemoryHandler(&errStore{err: errors.New("backend down")}))
	rr := doJSON(t, r, http.MethodPost, "/v1/store/memories", `{"content":"x"}`, "t1")
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on backend error; got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "backend down") {
		t.Errorf("expected error surfaced; got %s", rr.Body.String())
	}
}
