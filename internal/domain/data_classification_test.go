package domain

import "testing"

func TestDataClassPoliciesCoverKnownClasses(t *testing.T) {
	for _, class := range []DataClass{
		DataClassFinancial,
		DataClassOperational,
		DataClassUserContent,
		DataClassProviderPayload,
		DataClassArtifactMetadata,
		DataClassAnalyticsAggregate,
		DataClassTemporaryCache,
	} {
		t.Run(string(class), func(t *testing.T) {
			policy, ok := class.Policy()
			if !ok {
				t.Fatalf("missing policy for %s", class)
			}
			if policy.Class != class {
				t.Fatalf("policy class = %s, want %s", policy.Class, class)
			}
			if !policy.Retention.Valid() {
				t.Fatalf("invalid retention class %q for %s", policy.Retention, class)
			}
		})
	}
}

func TestFinancialDataIsProtectedFromGenericCleanup(t *testing.T) {
	if DataClassFinancial.AutoDeleteAllowed() {
		t.Fatal("financial data must not be auto-deletable")
	}
	if got := DataClassFinancial.MustPolicy().Retention; got != DataRetentionProtectedAudit {
		t.Fatalf("financial retention = %s, want %s", got, DataRetentionProtectedAudit)
	}
}

func TestSensitiveClassesRequireRedaction(t *testing.T) {
	for _, class := range []DataClass{DataClassUserContent, DataClassProviderPayload} {
		if !class.RequiresRedaction() {
			t.Fatalf("%s should require redaction", class)
		}
	}
}

func TestUnknownDataClassFallsBackToSafePolicy(t *testing.T) {
	unknown := DataClass("unknown")
	if unknown.Valid() {
		t.Fatal("unknown class reported valid")
	}
	policy := unknown.MustPolicy()
	if policy.AutoDeleteAllowed || !policy.RedactionRequired || !policy.ContainsPII {
		t.Fatalf("unknown fallback policy is not conservative enough: %+v", policy)
	}
	if policy.Retention != DataRetentionProtectedAudit {
		t.Fatalf("unknown retention = %s, want %s", policy.Retention, DataRetentionProtectedAudit)
	}
}

func TestArtifactTierValidation(t *testing.T) {
	for _, tier := range []ArtifactTier{
		ArtifactTierStandard,
		ArtifactTierFree,
		ArtifactTierPaid,
		ArtifactTierTemporary,
	} {
		if !tier.Valid() {
			t.Fatalf("artifact tier %q should be valid", tier)
		}
	}
	if ArtifactTier("vip").Valid() {
		t.Fatal("unknown artifact tier reported valid")
	}
}
