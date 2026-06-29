package domain

import "testing"

func TestOperatorRolePermissions(t *testing.T) {
	if !OperatorRoleOwner.HasPermission(OperatorPermissionRefundsManage) {
		t.Fatal("owner should manage refunds")
	}
	if !OperatorRoleAdmin.HasPermission(OperatorPermissionSystemStatusRead) {
		t.Fatal("admin should read system status")
	}
	if OperatorRoleAdmin.HasPermission(OperatorPermissionRefundsManage) {
		t.Fatal("admin should not manage refunds; finance owns payment/refund operations")
	}
	if !OperatorRoleSupport.HasPermission(OperatorPermissionJobsSafeRead) {
		t.Fatal("support should read safe job DTOs")
	}
	if OperatorRoleSupport.HasPermission(OperatorPermissionPaymentsSafeRead) {
		t.Fatal("support should not read payment surfaces")
	}
	if !OperatorRoleFinance.HasPermission(OperatorPermissionPaymentsSafeRead) {
		t.Fatal("finance should read safe payment DTOs")
	}
	if OperatorRoleFinance.HasPermission(OperatorPermissionJobsTriage) {
		t.Fatal("finance should not triage jobs")
	}
}

func TestOperatorRoleDefinitionsAreCopies(t *testing.T) {
	definitions := OperatorRoleDefinitions()
	if len(definitions) == 0 {
		t.Fatal("expected role definitions")
	}
	definitions[0].Permissions[0] = "mutated"
	definitions[0].DataBoundaries[0] = "mutated"
	definitions[0].Forbidden[0] = "mutated"

	fresh := OperatorRoleDefinitions()
	if fresh[0].Permissions[0] == "mutated" {
		t.Fatal("permissions slice was not copied")
	}
	if fresh[0].DataBoundaries[0] == "mutated" {
		t.Fatal("data boundary slice was not copied")
	}
	if fresh[0].Forbidden[0] == "mutated" {
		t.Fatal("forbidden slice was not copied")
	}
}

func TestParseOperatorRole(t *testing.T) {
	role, ok := ParseOperatorRole("finance")
	if !ok || role != OperatorRoleFinance {
		t.Fatalf("expected finance role, got %q ok=%v", role, ok)
	}
	if _, ok := ParseOperatorRole("super-admin"); ok {
		t.Fatal("unexpected role should be rejected")
	}
}
