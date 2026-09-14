package websession

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vk-ai-aggregator/internal/service/productcatalog"
)

func TestUnifiedModelsEndpointRequiresSessionAndReturnsTypedCatalog(t *testing.T) {
	h, _, _ := newTestHandler(t)
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/web/v1/models", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected auth boundary, got %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.listModels(w, httptest.NewRequest(http.MethodGet, "/web/v1/models", nil))
	var c productcatalog.WorkspaceModelList
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || json.Unmarshal(w.Body.Bytes(), &c) != nil || c.DefaultModelID == "" || len(c.Items) == 0 {
		t.Fatalf("invalid shared catalog: status=%d", w.Code)
	}
}
