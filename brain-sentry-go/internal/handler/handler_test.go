package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/integraltech/brainsentry/internal/dto"
	"github.com/integraltech/brainsentry/internal/service"
)

// --- Auth Handler Tests ---

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	h := NewAuthHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{invalid json`))
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestAuthHandler_Login_MissingEmail(t *testing.T) {
	h := NewAuthHandler(nil)
	body := `{"password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "email and password are required")
}

func TestAuthHandler_Login_MissingPassword(t *testing.T) {
	h := NewAuthHandler(nil)
	body := `{"email":"user@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "email and password are required")
}

func TestAuthHandler_Login_EmptyBody(t *testing.T) {
	h := NewAuthHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "email and password are required")
}

func TestAuthHandler_Login_EmptyBodyStream(t *testing.T) {
	h := NewAuthHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(""))
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestAuthHandler_Logout(t *testing.T) {
	h := NewAuthHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	rr := httptest.NewRecorder()

	h.Logout(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["message"] != "logged out successfully" {
		t.Errorf("unexpected message: %s", resp["message"])
	}
}

func TestAuthHandler_Refresh_InvalidJSON(t *testing.T) {
	h := NewAuthHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", strings.NewReader(`{bad`))
	rr := httptest.NewRecorder()

	h.Refresh(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestAuthHandler_Refresh_MissingToken(t *testing.T) {
	h := NewAuthHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	h.Refresh(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "refreshToken is required")
}

// --- Memory Handler Tests ---

func TestMemoryHandler_Create_InvalidJSON(t *testing.T) {
	h := NewMemoryHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/memories", strings.NewReader(`{invalid`))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestMemoryHandler_Create_MissingContent(t *testing.T) {
	h := NewMemoryHandler(nil, nil)
	body := `{"summary":"no content here"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/memories", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "content is required")
}

func TestMemoryHandler_Create_EmptyBody(t *testing.T) {
	h := NewMemoryHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/memories", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "content is required")
}

func TestMemoryHandler_Update_InvalidJSON(t *testing.T) {
	h := NewMemoryHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPut, "/v1/memories/abc", strings.NewReader(`{bad json`))
	req = withChiParam(req, "id", "abc")
	rr := httptest.NewRecorder()

	h.Update(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestMemoryHandler_Search_InvalidJSON(t *testing.T) {
	h := NewMemoryHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/search", strings.NewReader(`{bad`))
	rr := httptest.NewRecorder()

	h.Search(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestMemoryHandler_Search_MissingQuery(t *testing.T) {
	h := NewMemoryHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/search", strings.NewReader(`{"limit":10}`))
	rr := httptest.NewRecorder()

	h.Search(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "query is required unless sourceReference or metadata is given")
}

// The mirror of the above: an exact lookup carries its own target, so it must
// NOT be rejected for having no query (RFC-014 fatia 1).
func TestMemoryHandler_Search_ExactFilterNeedsNoQuery(t *testing.T) {
	h := NewMemoryHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/search",
		strings.NewReader(`{"sourceReference":"decisao:123"}`))
	rr := httptest.NewRecorder()

	// The service is nil here, so reaching it panics — which is exactly the
	// signal we want: getting past validation means the request was accepted.
	defer func() {
		if recover() == nil && rr.Code == http.StatusBadRequest {
			t.Error("an exact lookup must not be rejected for missing query")
		}
	}()
	h.Search(rr, req)
}

func TestMemoryHandler_Feedback_InvalidJSON(t *testing.T) {
	h := NewMemoryHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/abc/feedback", strings.NewReader(`{bad`))
	req = withChiParam(req, "id", "abc")
	rr := httptest.NewRecorder()

	h.Feedback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

// --- Correction Handler Tests ---

func TestCorrectionHandler_Flag_InvalidJSON(t *testing.T) {
	h := NewCorrectionHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/abc/flag", strings.NewReader(`{bad`))
	req = withChiParam(req, "id", "abc")
	rr := httptest.NewRecorder()

	h.Flag(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestCorrectionHandler_Flag_MissingReason(t *testing.T) {
	h := NewCorrectionHandler(nil)
	body := `{"correctedContent":"fixed content"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/abc/flag", strings.NewReader(body))
	req = withChiParam(req, "id", "abc")
	rr := httptest.NewRecorder()

	h.Flag(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "reason is required")
}

func TestCorrectionHandler_Flag_EmptyBody(t *testing.T) {
	h := NewCorrectionHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/abc/flag", strings.NewReader(`{}`))
	req = withChiParam(req, "id", "abc")
	rr := httptest.NewRecorder()

	h.Flag(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "reason is required")
}

func TestCorrectionHandler_Review_InvalidJSON(t *testing.T) {
	h := NewCorrectionHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/abc/review", strings.NewReader(`{bad`))
	req = withChiParam(req, "id", "abc")
	rr := httptest.NewRecorder()

	h.Review(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestCorrectionHandler_Review_InvalidAction(t *testing.T) {
	h := NewCorrectionHandler(nil)
	body := `{"action":"delete"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/abc/review", strings.NewReader(body))
	req = withChiParam(req, "id", "abc")
	rr := httptest.NewRecorder()

	h.Review(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "action must be 'approve' or 'reject'")
}

func TestCorrectionHandler_Review_EmptyAction(t *testing.T) {
	h := NewCorrectionHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/abc/review", strings.NewReader(`{}`))
	req = withChiParam(req, "id", "abc")
	rr := httptest.NewRecorder()

	h.Review(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "action must be 'approve' or 'reject'")
}

func TestCorrectionHandler_Review_ValidApproveAction(t *testing.T) {
	// With a nil service, this will panic on the service call.
	// We only test that valid action values pass the validation layer.
	// This test is intentionally skipped at service call.
	// We cannot test the "approve" path without a real service; instead
	// verify that "reject" is also recognized as valid.
	// We do this by confirming the invalid-action check does not trigger.
	// Since we have nil service we rely on the panic recovery below.
	t.Run("approve is valid action - passes validation", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// Panic happened in service call (nil pointer) - that means
				// validation passed, which is what we want to confirm.
			}
		}()
		h := NewCorrectionHandler(nil)
		body := `{"action":"approve"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/memories/abc/review", strings.NewReader(body))
		req = withChiParam(req, "id", "abc")
		rr := httptest.NewRecorder()
		h.Review(rr, req)
		// If we get here without panic the handler returned an error itself (e.g. 500)
		if rr.Code == http.StatusBadRequest {
			t.Error("approve should not trigger a 400 bad request")
		}
	})
}

func TestCorrectionHandler_Rollback_InvalidJSON(t *testing.T) {
	h := NewCorrectionHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/abc/rollback", strings.NewReader(`{bad`))
	req = withChiParam(req, "id", "abc")
	rr := httptest.NewRecorder()

	h.Rollback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestCorrectionHandler_Rollback_MissingVersion(t *testing.T) {
	h := NewCorrectionHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/abc/rollback", strings.NewReader(`{}`))
	req = withChiParam(req, "id", "abc")
	rr := httptest.NewRecorder()

	h.Rollback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "targetVersion is required")
}

func TestCorrectionHandler_Rollback_ZeroVersion(t *testing.T) {
	h := NewCorrectionHandler(nil)
	body := `{"targetVersion":0}`
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/abc/rollback", strings.NewReader(body))
	req = withChiParam(req, "id", "abc")
	rr := httptest.NewRecorder()

	h.Rollback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for zero version, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "targetVersion is required")
}

func TestCorrectionHandler_Rollback_NegativeVersion(t *testing.T) {
	h := NewCorrectionHandler(nil)
	body := `{"targetVersion":-1}`
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/abc/rollback", strings.NewReader(body))
	req = withChiParam(req, "id", "abc")
	rr := httptest.NewRecorder()

	h.Rollback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for negative version, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "targetVersion is required")
}

func TestCorrectionHandler_Rollback_VersionFromQueryParam(t *testing.T) {
	// When targetVersion is missing from body but provided as query param,
	// it should pass validation and proceed to the service call.
	defer func() {
		if r := recover(); r != nil {
			// Panic in service call (nil pointer) confirms validation passed.
		}
	}()
	h := NewCorrectionHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/abc/rollback?version=3", strings.NewReader(`{}`))
	req = withChiParam(req, "id", "abc")
	rr := httptest.NewRecorder()

	h.Rollback(rr, req)

	// If no panic and we reach here with 400, the query param wasn't picked up.
	if rr.Code == http.StatusBadRequest {
		t.Error("version query param should satisfy targetVersion requirement")
	}
}

// --- Stats Handler Tests ---

func TestStatsHandler_HealthStats(t *testing.T) {
	h := NewStatsHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/stats/health", nil)
	rr := httptest.NewRecorder()

	h.HealthStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "UP" {
		t.Errorf("expected status 'UP', got '%s'", resp["status"])
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
	}
}

// --- Swagger Handler Tests ---

func TestSwaggerSpec_ReturnsValidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/swagger.json", nil)
	rr := httptest.NewRecorder()

	SwaggerSpec(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
	}

	var spec map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&spec); err != nil {
		t.Fatalf("SwaggerSpec response is not valid JSON: %v", err)
	}
}

func TestSwaggerSpec_HasRequiredFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/swagger.json", nil)
	rr := httptest.NewRecorder()

	SwaggerSpec(rr, req)

	var spec map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&spec); err != nil {
		t.Fatalf("failed to decode swagger spec: %v", err)
	}

	requiredKeys := []string{"openapi", "info", "paths", "tags"}
	for _, key := range requiredKeys {
		if _, ok := spec[key]; !ok {
			t.Errorf("swagger spec missing required field: %s", key)
		}
	}
}

func TestSwaggerSpec_OpenAPIVersion(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/swagger.json", nil)
	rr := httptest.NewRecorder()

	SwaggerSpec(rr, req)

	var spec map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&spec); err != nil {
		t.Fatalf("failed to decode swagger spec: %v", err)
	}

	version, ok := spec["openapi"].(string)
	if !ok {
		t.Fatal("openapi field is not a string")
	}
	if version != "3.0.3" {
		t.Errorf("expected openapi version '3.0.3', got '%s'", version)
	}
}

func TestSwaggerSpec_HasPaths(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/swagger.json", nil)
	rr := httptest.NewRecorder()

	SwaggerSpec(rr, req)

	var spec map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&spec); err != nil {
		t.Fatalf("failed to decode swagger spec: %v", err)
	}

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("paths field is missing or not an object")
	}

	expectedPaths := []string{
		"/v1/auth/login",
		"/v1/memories",
		"/v1/memories/{id}",
		"/v1/intercept",
		"/health",
	}
	for _, path := range expectedPaths {
		if _, ok := paths[path]; !ok {
			t.Errorf("swagger spec missing expected path: %s", path)
		}
	}
}

// --- Interception Handler Tests ---

func TestInterceptionHandler_Intercept_InvalidJSON(t *testing.T) {
	h := NewInterceptionHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/intercept", strings.NewReader(`{bad`))
	rr := httptest.NewRecorder()

	h.Intercept(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestInterceptionHandler_Intercept_MissingPrompt(t *testing.T) {
	h := NewInterceptionHandler(nil)
	body := `{"userId":"u1","sessionId":"s1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/intercept", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.Intercept(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "prompt is required")
}

func TestInterceptionHandler_Intercept_EmptyBody(t *testing.T) {
	h := NewInterceptionHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/intercept", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	h.Intercept(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "prompt is required")
}

// --- Learning Lifecycle Handler Tests ---

func TestReconciliationHandler_Reconcile_ServiceUnavailable(t *testing.T) {
	h := NewReconciliationHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/reconcile", strings.NewReader(`{"content":"fact"}`))
	rr := httptest.NewRecorder()

	h.Reconcile(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "reconciliation service is not available")
}

func TestReconciliationHandler_Reconcile_InvalidJSON(t *testing.T) {
	h := NewReconciliationHandler(&service.ReconciliationService{})
	req := httptest.NewRequest(http.MethodPost, "/v1/reconcile", strings.NewReader(`{bad`))
	rr := httptest.NewRecorder()

	h.Reconcile(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestReconciliationHandler_Reconcile_MissingContent(t *testing.T) {
	h := NewReconciliationHandler(&service.ReconciliationService{})
	req := httptest.NewRequest(http.MethodPost, "/v1/reconcile", strings.NewReader(`{"sessionId":"s1"}`))
	rr := httptest.NewRecorder()

	h.Reconcile(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "content is required")
}

func TestConsolidationHandler_Consolidate_ServiceUnavailable(t *testing.T) {
	h := NewConsolidationHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/consolidate", strings.NewReader(`{"similarityThreshold":0.9}`))
	rr := httptest.NewRecorder()

	h.Consolidate(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "consolidation service is not available")
}

func TestSemanticMemoryHandler_Consolidate_ServiceUnavailable(t *testing.T) {
	h := NewSemanticMemoryHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/semantic/consolidate?minMemories=3", nil)
	rr := httptest.NewRecorder()

	h.Consolidate(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "semantic memory service is not available")
}

func TestAutoForgetHandler_Run_ServiceUnavailable(t *testing.T) {
	h := NewAutoForgetHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/auto-forget?dryRun=true", nil)
	rr := httptest.NewRecorder()

	h.Run(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "auto-forget service is not available")
}

func TestActivationHandler_Activate_ServiceUnavailable(t *testing.T) {
	h := NewActivationHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/memories/activate", strings.NewReader(`{"seedIds":["m1"]}`))
	rr := httptest.NewRecorder()

	h.Activate(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "activation service is not available")
}

// --- Relationship Handler Tests ---

func TestRelationshipHandler_Create_InvalidJSON(t *testing.T) {
	h := NewRelationshipHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/relationships", strings.NewReader(`{bad`))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestRelationshipHandler_Create_MissingFromMemoryID(t *testing.T) {
	h := NewRelationshipHandler(nil, nil)
	body := `{"toMemoryId":"mem-2","type":"related_to"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/relationships", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "fromMemoryId and toMemoryId are required")
}

func TestRelationshipHandler_Create_MissingToMemoryID(t *testing.T) {
	h := NewRelationshipHandler(nil, nil)
	body := `{"fromMemoryId":"mem-1","type":"related_to"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/relationships", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "fromMemoryId and toMemoryId are required")
}

func TestRelationshipHandler_Create_BothMemoryIDsMissing(t *testing.T) {
	h := NewRelationshipHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/relationships", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "fromMemoryId and toMemoryId are required")
}

func TestRelationshipHandler_GetBetween_MissingFromParam(t *testing.T) {
	h := NewRelationshipHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/relationships/between?to=mem-2", nil)
	rr := httptest.NewRecorder()

	h.GetBetween(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "from and to query params are required")
}

func TestRelationshipHandler_GetBetween_MissingToParam(t *testing.T) {
	h := NewRelationshipHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/relationships/between?from=mem-1", nil)
	rr := httptest.NewRecorder()

	h.GetBetween(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "from and to query params are required")
}

func TestRelationshipHandler_GetBetween_BothParamsMissing(t *testing.T) {
	h := NewRelationshipHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/relationships/between", nil)
	rr := httptest.NewRecorder()

	h.GetBetween(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "from and to query params are required")
}

func TestRelationshipHandler_DeleteBetween_MissingParams(t *testing.T) {
	h := NewRelationshipHandler(nil, nil)
	req := httptest.NewRequest(http.MethodDelete, "/v1/relationships/between", nil)
	rr := httptest.NewRecorder()

	h.DeleteBetween(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "from and to query params are required")
}

func TestRelationshipHandler_CreateBidirectional_InvalidJSON(t *testing.T) {
	h := NewRelationshipHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/relationships/bidirectional", strings.NewReader(`{bad`))
	rr := httptest.NewRecorder()

	h.CreateBidirectional(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestRelationshipHandler_UpdateStrength_InvalidJSON(t *testing.T) {
	h := NewRelationshipHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPut, "/v1/relationships/rel-1/strength", strings.NewReader(`{bad`))
	req = withChiParam(req, "relationshipId", "rel-1")
	rr := httptest.NewRecorder()

	h.UpdateStrength(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

// --- NoteTaking Handler Tests ---

func TestNoteTakingHandler_AnalyzeSession_InvalidJSON(t *testing.T) {
	h := NewNoteTakingHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/notes/analyze", strings.NewReader(`{bad`))
	rr := httptest.NewRecorder()

	h.AnalyzeSession(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestNoteTakingHandler_AnalyzeSession_MissingSessionID(t *testing.T) {
	h := NewNoteTakingHandler(nil)
	body := `{"tenantId":"t1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/notes/analyze", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.AnalyzeSession(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "sessionId is required")
}

func TestNoteTakingHandler_AnalyzeSession_EmptyBody(t *testing.T) {
	h := NewNoteTakingHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/notes/analyze", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	h.AnalyzeSession(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "sessionId is required")
}

func TestNoteTakingHandler_CreateHindsight_InvalidJSON(t *testing.T) {
	h := NewNoteTakingHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/notes/hindsight", strings.NewReader(`{bad`))
	rr := httptest.NewRecorder()

	h.CreateHindsight(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestNoteTakingHandler_CreateHindsight_MissingRequiredFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "missing all required fields",
			body: `{}`,
		},
		{
			name: "missing errorType",
			body: `{"sessionId":"s1","errorMessage":"something failed"}`,
		},
		{
			name: "missing sessionId",
			body: `{"errorType":"runtime","errorMessage":"something failed"}`,
		},
		{
			name: "missing errorMessage",
			body: `{"sessionId":"s1","errorType":"runtime"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewNoteTakingHandler(nil)
			req := httptest.NewRequest(http.MethodPost, "/v1/notes/hindsight", strings.NewReader(tc.body))
			rr := httptest.NewRecorder()

			h.CreateHindsight(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("[%s] expected status 400, got %d", tc.name, rr.Code)
			}
		})
	}
}

// --- Compression Handler Tests ---

func TestCompressionHandler_Compress_InvalidJSON(t *testing.T) {
	h := NewCompressionHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/compression/compress", strings.NewReader(`{bad`))
	rr := httptest.NewRecorder()

	h.Compress(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestCompressionHandler_Compress_EmptyMessages(t *testing.T) {
	h := NewCompressionHandler(nil)
	body := `{"messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/compression/compress", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.Compress(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "messages are required")
}

func TestCompressionHandler_Compress_MissingMessages(t *testing.T) {
	h := NewCompressionHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/compression/compress", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	h.Compress(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "messages are required")
}

// --- Entity Graph Handler Tests ---

func TestEntityGraphHandler_SearchEntities_MissingQuery(t *testing.T) {
	h := NewEntityGraphHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/entity-graph/search", nil)
	rr := httptest.NewRecorder()

	h.SearchEntities(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "query parameter 'q' is required")
}

func TestEntityGraphHandler_SearchEntities_EmptyQuery(t *testing.T) {
	h := NewEntityGraphHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/entity-graph/search?q=", nil)
	rr := httptest.NewRecorder()

	h.SearchEntities(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "query parameter 'q' is required")
}

func TestEntityGraphHandler_BatchExtract_InvalidJSON(t *testing.T) {
	h := NewEntityGraphHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/entity-graph/extract-batch", strings.NewReader(`{bad`))
	rr := httptest.NewRecorder()

	h.BatchExtract(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

// --- Audit Handler Tests ---

func TestAuditHandler_ByDateRange_InvalidFromDate(t *testing.T) {
	h := NewAuditHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/audit-logs/by-date-range?from=not-a-date&to=2024-01-01T00:00:00Z", nil)
	rr := httptest.NewRecorder()

	h.ByDateRange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid 'from' date format, use RFC3339")
}

func TestAuditHandler_ByDateRange_InvalidToDate(t *testing.T) {
	h := NewAuditHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/audit-logs/by-date-range?from=2024-01-01T00:00:00Z&to=not-a-date", nil)
	rr := httptest.NewRecorder()

	h.ByDateRange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid 'to' date format, use RFC3339")
}

func TestAuditHandler_ByDateRange_MissingBothDates(t *testing.T) {
	h := NewAuditHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/audit-logs/by-date-range", nil)
	rr := httptest.NewRecorder()

	h.ByDateRange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

// --- User Handler Tests ---

func TestUserHandler_Create_InvalidJSON(t *testing.T) {
	h := NewUserHandler(nil, nil, 10)
	req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(`{bad`))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestUserHandler_Create_MissingEmail(t *testing.T) {
	h := NewUserHandler(nil, nil, 10)
	body := `{"password":"secret","tenantId":"t1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "email, password, and tenantId are required")
}

func TestUserHandler_Create_MissingPassword(t *testing.T) {
	h := NewUserHandler(nil, nil, 10)
	body := `{"email":"user@example.com","tenantId":"t1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "email, password, and tenantId are required")
}

func TestUserHandler_Create_MissingTenantID(t *testing.T) {
	h := NewUserHandler(nil, nil, 10)
	body := `{"email":"user@example.com","password":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "email, password, and tenantId are required")
}

func TestUserHandler_Create_EmptyBody(t *testing.T) {
	h := NewUserHandler(nil, nil, 10)
	req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "email, password, and tenantId are required")
}

// --- Tenant Handler Tests ---

func TestTenantHandler_Create_InvalidJSON(t *testing.T) {
	h := NewTenantHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/tenants", strings.NewReader(`{bad`))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "invalid request body")
}

func TestTenantHandler_Create_MissingName(t *testing.T) {
	h := NewTenantHandler(nil)
	body := `{"slug":"my-tenant"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/tenants", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "name and slug are required")
}

func TestTenantHandler_Create_MissingSlug(t *testing.T) {
	h := NewTenantHandler(nil)
	body := `{"name":"My Tenant"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/tenants", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "name and slug are required")
}

func TestTenantHandler_Create_EmptyBody(t *testing.T) {
	h := NewTenantHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/tenants", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
	assertErrorMessage(t, rr, "name and slug are required")
}

func TestTenantHandler_Update_InvalidJSON(t *testing.T) {
	// Update first tries to fetch the tenant (will panic with nil repo).
	// We only verify the JSON decode error which happens before any service call.
	// With a nil tenantRepo the FindByID call will panic before we reach JSON decode,
	// so this test demonstrates the architecture; the 400 path is not reachable
	// without a real repo. Skip this sub-case and document the design.
	t.Skip("TenantHandler.Update requires a working repo before JSON decode; integration test needed")
}

// --- Content-Type header tests ---

func TestHandlers_ReturnApplicationJSON(t *testing.T) {
	tests := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		req     *http.Request
	}{
		{
			name:    "Health",
			handler: Health,
			req:     httptest.NewRequest(http.MethodGet, "/health", nil),
		},
		{
			name: "StatsHealthStats",
			handler: func(w http.ResponseWriter, r *http.Request) {
				NewStatsHandler(nil, nil).HealthStats(w, r)
			},
			req: httptest.NewRequest(http.MethodGet, "/v1/stats/health", nil),
		},
		{
			name:    "SwaggerSpec",
			handler: SwaggerSpec,
			req:     httptest.NewRequest(http.MethodGet, "/swagger.json", nil),
		},
		{
			name: "AuthLogout",
			handler: func(w http.ResponseWriter, r *http.Request) {
				NewAuthHandler(nil).Logout(w, r)
			},
			req: httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil),
		},
		{
			name: "AuthLogin_BadRequest",
			handler: func(w http.ResponseWriter, r *http.Request) {
				NewAuthHandler(nil).Login(w, r)
			},
			req: httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{}`)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			tt.handler(rr, tt.req)
			ct := rr.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("[%s] expected Content-Type 'application/json', got '%s'", tt.name, ct)
			}
		})
	}
}

// --- Error response structure tests ---

func TestErrorResponse_Structure(t *testing.T) {
	h := NewAuthHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	var resp dto.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if resp.Status != http.StatusBadRequest {
		t.Errorf("expected status 400 in body, got %d", resp.Status)
	}
	if resp.Error == "" {
		t.Error("error field should not be empty")
	}
	if resp.Message == "" {
		t.Error("message field should not be empty")
	}
	if resp.Error != "Bad Request" {
		t.Errorf("expected error 'Bad Request', got '%s'", resp.Error)
	}
}

// --- Route parameter extraction tests ---

func TestChiURLParam_MemoryID(t *testing.T) {
	// Verify that our withChiParam helper correctly injects URL params
	// so handlers can read them via chi.URLParam.
	req := httptest.NewRequest(http.MethodGet, "/v1/memories/test-id-123", nil)
	req = withChiParam(req, "id", "test-id-123")

	id := chi.URLParam(req, "id")
	if id != "test-id-123" {
		t.Errorf("expected chi URL param 'test-id-123', got '%s'", id)
	}
}

func TestChiURLParam_MultipleParts(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/entity-graph/memory/mem-abc/entities", nil)
	req = withChiParam(req, "memoryId", "mem-abc")

	memID := chi.URLParam(req, "memoryId")
	if memID != "mem-abc" {
		t.Errorf("expected chi URL param 'mem-abc', got '%s'", memID)
	}
}

// --- Helpers ---

// withChiParam injects a chi route parameter into the request context.
func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// assertErrorMessage decodes the response body as a dto.ErrorResponse and checks the message.
func assertErrorMessage(t *testing.T, rr *httptest.ResponseRecorder, want string) {
	t.Helper()
	var resp dto.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if resp.Message != want {
		t.Errorf("expected message %q, got %q", want, resp.Message)
	}
}

// Enum inválido tem que dar 400 e não 201. Antes, o valor era aceito, gravado,
// e depois não casava com filtro nenhum — o dado existia e era invisível.
func TestMemoryHandler_Create_RejectsUnknownEnums(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"importance", `{"content":"x","importance":"NORMAL"}`, "importance"},
		{"category", `{"content":"x","category":"INVENTADA"}`, "category"},
		{"provenance", `{"content":"x","provenance":"CHUTE"}`, "provenance"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewMemoryHandler(nil, nil)
			req := httptest.NewRequest(http.MethodPost, "/v1/memories", strings.NewReader(tc.body))
			rr := httptest.NewRecorder()

			h.Create(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("esperava 400 para %s invalido, veio %d", tc.name, rr.Code)
			}
			if !strings.Contains(rr.Body.String(), tc.want) {
				t.Errorf("a mensagem deveria dizer qual campo esta errado: %s", rr.Body.String())
			}
		})
	}
}

// "NORMAL" é o valor que a própria documentação chegou a sugerir por engano —
// vale um caso explícito para ele não voltar.
func TestMemoryHandler_Create_AcceptsValidEnums(t *testing.T) {
	h := NewMemoryHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/memories",
		strings.NewReader(`{"content":"x","importance":"IMPORTANT","category":"DECISION","provenance":"EXPLICIT"}`))
	rr := httptest.NewRecorder()

	// O service é nil, então passar da validação chega a um panic — que é
	// justamente o sinal de que os enums foram aceitos.
	defer func() {
		if recover() == nil && rr.Code == http.StatusBadRequest {
			t.Error("enums validos foram rejeitados")
		}
	}()
	h.Create(rr, req)
}
