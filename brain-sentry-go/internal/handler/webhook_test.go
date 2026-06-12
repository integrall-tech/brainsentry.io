package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/service"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

func newWebhookHandler() (*WebhookHandler, *service.WebhookService) {
	svc := service.NewWebhookService(nil) // in-memory mode
	return NewWebhookHandler(svc), svc
}

func webhookRequest(method, path, body, tenantID string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	return req.WithContext(tenant.WithTenant(req.Context(), tenantID))
}

func TestWebhookHandler_RegisterCreated(t *testing.T) {
	h, _ := newWebhookHandler()

	req := webhookRequest(http.MethodPost, "/v1/webhooks",
		`{"url":"https://example.com/hook","secret":"s","events":["memory.created"]}`, "tenant-a")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var wh domain.Webhook
	if err := json.Unmarshal(rec.Body.Bytes(), &wh); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if wh.TenantID != "tenant-a" {
		t.Errorf("expected tenant from context, got %q", wh.TenantID)
	}
	if wh.URL != "https://example.com/hook" || !wh.Active {
		t.Errorf("unexpected webhook payload: %+v", wh)
	}
}

func TestWebhookHandler_RegisterMissingURL(t *testing.T) {
	h, _ := newWebhookHandler()

	req := webhookRequest(http.MethodPost, "/v1/webhooks", `{"secret":"s"}`, "tenant-a")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing url, got %d", rec.Code)
	}
}

func TestWebhookHandler_RegisterInvalidJSON(t *testing.T) {
	h, _ := newWebhookHandler()

	req := webhookRequest(http.MethodPost, "/v1/webhooks", `{not json`, "tenant-a")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", rec.Code)
	}
}

func TestWebhookHandler_ListIsTenantScoped(t *testing.T) {
	h, svc := newWebhookHandler()
	svc.Register(tenant.WithTenant(httptest.NewRequest("GET", "/", nil).Context(), "tenant-a"),
		"tenant-a", "https://a.example.com", "", nil)
	svc.Register(tenant.WithTenant(httptest.NewRequest("GET", "/", nil).Context(), "tenant-b"),
		"tenant-b", "https://b.example.com", "", nil)

	req := webhookRequest(http.MethodGet, "/v1/webhooks", "", "tenant-a")
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var list []domain.Webhook
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if len(list) != 1 || list[0].TenantID != "tenant-a" {
		t.Errorf("expected only tenant-a webhooks, got %+v", list)
	}
}

func TestWebhookHandler_UnregisterFlow(t *testing.T) {
	h, svc := newWebhookHandler()
	wh := svc.Register(tenant.WithTenant(httptest.NewRequest("GET", "/", nil).Context(), "tenant-a"),
		"tenant-a", "https://a.example.com", "", nil)

	req := webhookRequest(http.MethodDelete, "/v1/webhooks/"+wh.ID, "", "tenant-a")
	req = withChiParam(req, "id", wh.ID)
	rec := httptest.NewRecorder()
	h.Unregister(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 deleting existing webhook, got %d", rec.Code)
	}

	// Second delete: gone.
	rec = httptest.NewRecorder()
	h.Unregister(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 deleting missing webhook, got %d", rec.Code)
	}
}

func TestWebhookHandler_DeliveriesEmptyOK(t *testing.T) {
	h, svc := newWebhookHandler()
	wh := svc.Register(tenant.WithTenant(httptest.NewRequest("GET", "/", nil).Context(), "tenant-a"),
		"tenant-a", "https://a.example.com", "", nil)

	req := webhookRequest(http.MethodGet, "/v1/webhooks/"+wh.ID+"/deliveries", "", "tenant-a")
	req = withChiParam(req, "id", wh.ID)
	rec := httptest.NewRecorder()

	h.Deliveries(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}
