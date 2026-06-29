package admin

import (
	"net/http"
	"time"

	"vk-ai-aggregator/internal/domain"
)

func (h *Handler) getOperatorAccess(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, OperatorAccessDTO{
		GeneratedAt:     time.Now().UTC(),
		CurrentAuthMode: "admin_token",
		EffectiveRole:   string(domain.OperatorRoleOwner),
		GlobalBoundaries: []string{
			string(domain.OperatorDataBoundaryBackendAPIOnly),
			string(domain.OperatorDataBoundaryNoDirectSQL),
			string(domain.OperatorDataBoundarySafeDTOOnly),
			string(domain.OperatorDataBoundaryPIIRedacted),
			string(domain.OperatorDataBoundaryPromptRedacted),
			string(domain.OperatorDataBoundaryProviderRedacted),
			string(domain.OperatorDataBoundaryPaymentRedacted),
			string(domain.OperatorDataBoundaryPrivateURLsHidden),
			string(domain.OperatorDataBoundaryPaginated),
		},
		Roles: operatorRoleAccessDTOs(domain.OperatorRoleDefinitions()),
		Notes: []string{
			"Current ADMIN_TOKEN access is treated as owner/break-glass until per-operator identity is introduced.",
			"UI role checks are advisory; backend handlers/services remain the enforcement boundary.",
			"Operator surfaces must use backend API and safe DTOs only; direct SQL and raw payload views are out of scope.",
		},
	})
}

func operatorRoleAccessDTOs(definitions []domain.OperatorRoleDefinition) []OperatorRoleAccessDTO {
	out := make([]OperatorRoleAccessDTO, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, OperatorRoleAccessDTO{
			Role:           string(definition.Role),
			Description:    definition.Description,
			Permissions:    operatorPermissionStrings(definition.Permissions),
			DataBoundaries: operatorBoundaryStrings(definition.DataBoundaries),
			Forbidden:      append([]string(nil), definition.Forbidden...),
		})
	}
	return out
}

func operatorPermissionStrings(permissions []domain.OperatorPermission) []string {
	out := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		out = append(out, string(permission))
	}
	return out
}

func operatorBoundaryStrings(boundaries []domain.OperatorDataBoundary) []string {
	out := make([]string, 0, len(boundaries))
	for _, boundary := range boundaries {
		out = append(out, string(boundary))
	}
	return out
}
