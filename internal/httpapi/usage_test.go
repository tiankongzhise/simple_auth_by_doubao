package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleUsage(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/public/usage", nil)
	rec := httptest.NewRecorder()

	(&Server{}).handleUsage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var usage apiUsage
	if err := json.NewDecoder(rec.Body).Decode(&usage); err != nil {
		t.Fatalf("decode usage: %v", err)
	}
	if usage.ServiceName == "" {
		t.Fatalf("serviceName is empty")
	}
	if len(usage.Endpoints) == 0 {
		t.Fatalf("endpoints is empty")
	}
	if !hasEndpoint(usage.Endpoints, http.MethodPost, "/api/auth/verify") {
		t.Fatalf("missing /api/auth/verify endpoint")
	}
	if !hasEndpoint(usage.Endpoints, http.MethodPost, "/api/service-groups/token/latest") {
		t.Fatalf("missing /api/service-groups/token/latest endpoint")
	}
}

func hasEndpoint(endpoints []usageEndpoint, method string, path string) bool {
	for _, endpoint := range endpoints {
		if endpoint.Method == method && endpoint.Path == path {
			return true
		}
	}
	return false
}
