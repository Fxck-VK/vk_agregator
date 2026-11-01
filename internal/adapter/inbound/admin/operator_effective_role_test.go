package admin_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vk-ai-aggregator/internal/adapter/inbound/admin"
	"vk-ai-aggregator/internal/domain"
)

func TestOperatorAccessReportsConfiguredRoleRatherThanClientClaims(t *testing.T) {
	for _, role := range []domain.OperatorRole{domain.OperatorRoleOwner, domain.OperatorRoleAdmin, domain.OperatorRoleFinance, domain.OperatorRoleSupport, domain.OperatorRoleOperator} {
		t.Run(string(role), func(t *testing.T) {
			h := admin.NewHandler(admin.Config{Token: testAdminToken, DefaultRole: role}, admin.Deps{}).Routes()
			req := httptest.NewRequest(http.MethodGet, "/admin/access/operator?role=owner", nil)
			req.Header.Set("X-Admin-Token", testAdminToken)
			req.Header.Set("X-Operator-Role", "owner")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			var dto admin.OperatorAccessDTO
			if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &dto) != nil || dto.EffectiveRole != string(role) {
				t.Fatalf("role %s: status=%d effective=%s", role, rec.Code, dto.EffectiveRole)
			}
		})
	}
}
