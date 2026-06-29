package domain

// OperatorRole is a bounded backend role for operator/admin UI access.
type OperatorRole string

const (
	OperatorRoleOwner    OperatorRole = "owner"
	OperatorRoleAdmin    OperatorRole = "admin"
	OperatorRoleOperator OperatorRole = "operator"
	OperatorRoleSupport  OperatorRole = "support"
	OperatorRoleFinance  OperatorRole = "finance"
)

// OperatorPermission names one backend API capability. UI code may render these
// permissions, but enforcement belongs to backend handlers/services.
type OperatorPermission string

const (
	OperatorPermissionAccessRead       OperatorPermission = "operator_access:read"
	OperatorPermissionSystemStatusRead OperatorPermission = "system_status:read"
	OperatorPermissionJobsSafeRead     OperatorPermission = "jobs:safe_read"
	OperatorPermissionJobsTriage       OperatorPermission = "jobs:triage"
	OperatorPermissionQueueRead        OperatorPermission = "queue:read"
	OperatorPermissionDLQReplay        OperatorPermission = "dlq:replay"
	OperatorPermissionProviderHealth   OperatorPermission = "provider_health:read"
	OperatorPermissionRetentionRead    OperatorPermission = "retention:read"
	OperatorPermissionRetentionDryRun  OperatorPermission = "retention:dry_run"
	OperatorPermissionRetentionCleanup OperatorPermission = "retention:cleanup"
	OperatorPermissionAnalyticsRead    OperatorPermission = "analytics:read"
	OperatorPermissionUsersSafeRead    OperatorPermission = "users:safe_read"
	OperatorPermissionReferralsRead    OperatorPermission = "referrals:read"
	OperatorPermissionPaymentsSafeRead OperatorPermission = "payments:safe_read"
	OperatorPermissionRefundsManage    OperatorPermission = "refunds:manage"
	OperatorPermissionAuditRead        OperatorPermission = "audit:read"
	OperatorPermissionRolePolicyManage OperatorPermission = "role_policy:manage"
)

// OperatorDataBoundary describes data handling rules every operator UI surface
// must preserve.
type OperatorDataBoundary string

const (
	OperatorDataBoundaryBackendAPIOnly      OperatorDataBoundary = "backend_api_only"
	OperatorDataBoundaryNoDirectSQL         OperatorDataBoundary = "no_direct_sql"
	OperatorDataBoundarySafeDTOOnly         OperatorDataBoundary = "safe_dto_only"
	OperatorDataBoundaryPIIRedacted         OperatorDataBoundary = "pii_redacted"
	OperatorDataBoundaryPromptRedacted      OperatorDataBoundary = "prompt_redacted"
	OperatorDataBoundaryProviderRedacted    OperatorDataBoundary = "provider_payload_redacted"
	OperatorDataBoundaryPaymentRedacted     OperatorDataBoundary = "payment_payload_redacted"
	OperatorDataBoundaryPrivateURLsHidden   OperatorDataBoundary = "private_urls_hidden"
	OperatorDataBoundaryPaginated           OperatorDataBoundary = "paginated"
	OperatorDataBoundaryMutationAudited     OperatorDataBoundary = "mutation_audited"
	OperatorDataBoundaryFinancialLedgerOnly OperatorDataBoundary = "financial_ledger_only"
)

// OperatorRoleDefinition is the safe, versioned role contract for operator UI.
type OperatorRoleDefinition struct {
	Role           OperatorRole
	Description    string
	Permissions    []OperatorPermission
	DataBoundaries []OperatorDataBoundary
	Forbidden      []string
}

// Valid reports whether the role is part of the operator access contract.
func (r OperatorRole) Valid() bool {
	switch r {
	case OperatorRoleOwner, OperatorRoleAdmin, OperatorRoleOperator, OperatorRoleSupport, OperatorRoleFinance:
		return true
	default:
		return false
	}
}

// ParseOperatorRole normalizes a string into an OperatorRole.
func ParseOperatorRole(value string) (OperatorRole, bool) {
	role := OperatorRole(value)
	return role, role.Valid()
}

// HasPermission reports whether a role contains a permission in the current
// contract.
func (r OperatorRole) HasPermission(permission OperatorPermission) bool {
	for _, candidate := range OperatorPermissionsForRole(r) {
		if candidate == permission {
			return true
		}
	}
	return false
}

// OperatorPermissionsForRole returns a copy of the permissions for one role.
func OperatorPermissionsForRole(role OperatorRole) []OperatorPermission {
	for _, definition := range operatorRoleDefinitions {
		if definition.Role == role {
			return cloneOperatorPermissions(definition.Permissions)
		}
	}
	return nil
}

// OperatorRoleDefinitions returns a copy of the full role contract.
func OperatorRoleDefinitions() []OperatorRoleDefinition {
	out := make([]OperatorRoleDefinition, 0, len(operatorRoleDefinitions))
	for _, definition := range operatorRoleDefinitions {
		out = append(out, OperatorRoleDefinition{
			Role:           definition.Role,
			Description:    definition.Description,
			Permissions:    cloneOperatorPermissions(definition.Permissions),
			DataBoundaries: cloneOperatorBoundaries(definition.DataBoundaries),
			Forbidden:      append([]string(nil), definition.Forbidden...),
		})
	}
	return out
}

func cloneOperatorPermissions(in []OperatorPermission) []OperatorPermission {
	return append([]OperatorPermission(nil), in...)
}

func cloneOperatorBoundaries(in []OperatorDataBoundary) []OperatorDataBoundary {
	return append([]OperatorDataBoundary(nil), in...)
}

var commonOperatorBoundaries = []OperatorDataBoundary{
	OperatorDataBoundaryBackendAPIOnly,
	OperatorDataBoundaryNoDirectSQL,
	OperatorDataBoundarySafeDTOOnly,
	OperatorDataBoundaryPIIRedacted,
	OperatorDataBoundaryPromptRedacted,
	OperatorDataBoundaryProviderRedacted,
	OperatorDataBoundaryPaymentRedacted,
	OperatorDataBoundaryPrivateURLsHidden,
	OperatorDataBoundaryPaginated,
}

var operatorRoleDefinitions = []OperatorRoleDefinition{
	{
		Role:        OperatorRoleOwner,
		Description: "Full break-glass ownership of operator/admin policy and protected actions.",
		Permissions: []OperatorPermission{
			OperatorPermissionAccessRead,
			OperatorPermissionSystemStatusRead,
			OperatorPermissionJobsSafeRead,
			OperatorPermissionJobsTriage,
			OperatorPermissionQueueRead,
			OperatorPermissionDLQReplay,
			OperatorPermissionProviderHealth,
			OperatorPermissionRetentionRead,
			OperatorPermissionRetentionDryRun,
			OperatorPermissionRetentionCleanup,
			OperatorPermissionAnalyticsRead,
			OperatorPermissionUsersSafeRead,
			OperatorPermissionReferralsRead,
			OperatorPermissionPaymentsSafeRead,
			OperatorPermissionRefundsManage,
			OperatorPermissionAuditRead,
			OperatorPermissionRolePolicyManage,
		},
		DataBoundaries: append(append([]OperatorDataBoundary(nil), commonOperatorBoundaries...),
			OperatorDataBoundaryMutationAudited,
			OperatorDataBoundaryFinancialLedgerOnly,
		),
		Forbidden: []string{
			"direct SQL writes",
			"raw secrets/tokens/auth headers",
			"raw prompts/provider payloads/private artifact URLs",
			"direct balance mutation outside ledger",
		},
	},
	{
		Role:        OperatorRoleAdmin,
		Description: "System operator for health, queues, jobs, provider status, retention and audit visibility.",
		Permissions: []OperatorPermission{
			OperatorPermissionAccessRead,
			OperatorPermissionSystemStatusRead,
			OperatorPermissionJobsSafeRead,
			OperatorPermissionJobsTriage,
			OperatorPermissionQueueRead,
			OperatorPermissionDLQReplay,
			OperatorPermissionProviderHealth,
			OperatorPermissionRetentionRead,
			OperatorPermissionRetentionDryRun,
			OperatorPermissionRetentionCleanup,
			OperatorPermissionAnalyticsRead,
			OperatorPermissionUsersSafeRead,
			OperatorPermissionReferralsRead,
			OperatorPermissionAuditRead,
		},
		DataBoundaries: append(append([]OperatorDataBoundary(nil), commonOperatorBoundaries...),
			OperatorDataBoundaryMutationAudited,
		),
		Forbidden: []string{
			"finance refunds",
			"direct SQL",
			"raw PII/prompts/provider payloads",
		},
	},
	{
		Role:        OperatorRoleOperator,
		Description: "Runtime operator for job queues, provider health, DLQ triage and retention dry-runs.",
		Permissions: []OperatorPermission{
			OperatorPermissionAccessRead,
			OperatorPermissionSystemStatusRead,
			OperatorPermissionJobsSafeRead,
			OperatorPermissionJobsTriage,
			OperatorPermissionQueueRead,
			OperatorPermissionDLQReplay,
			OperatorPermissionProviderHealth,
			OperatorPermissionRetentionRead,
			OperatorPermissionRetentionDryRun,
			OperatorPermissionAnalyticsRead,
		},
		DataBoundaries: append(append([]OperatorDataBoundary(nil), commonOperatorBoundaries...),
			OperatorDataBoundaryMutationAudited,
		),
		Forbidden: []string{
			"payments/refunds",
			"user PII",
			"raw prompts/provider payloads",
		},
	},
	{
		Role:        OperatorRoleSupport,
		Description: "Support view for safe job/user troubleshooting without PII, payment payloads or provider internals.",
		Permissions: []OperatorPermission{
			OperatorPermissionAccessRead,
			OperatorPermissionJobsSafeRead,
			OperatorPermissionUsersSafeRead,
			OperatorPermissionReferralsRead,
		},
		DataBoundaries: commonOperatorBoundaries,
		Forbidden: []string{
			"payment/refund operations",
			"raw user identifiers and contact data",
			"raw prompts/provider payloads/private URLs",
		},
	},
	{
		Role:        OperatorRoleFinance,
		Description: "Finance view for safe payment/refund operations backed by ledger and provider verification.",
		Permissions: []OperatorPermission{
			OperatorPermissionAccessRead,
			OperatorPermissionPaymentsSafeRead,
			OperatorPermissionRefundsManage,
			OperatorPermissionUsersSafeRead,
			OperatorPermissionAuditRead,
		},
		DataBoundaries: append(append([]OperatorDataBoundary(nil), commonOperatorBoundaries...),
			OperatorDataBoundaryMutationAudited,
			OperatorDataBoundaryFinancialLedgerOnly,
		),
		Forbidden: []string{
			"raw provider payment payloads",
			"direct balance mutation",
			"job prompts and provider media payloads",
		},
	},
}
