package domain

// DataClass identifies the security/retention class of persisted data. Values
// are internal policy labels, not public API fields.
type DataClass string

const (
	DataClassFinancial          DataClass = "financial"
	DataClassOperational        DataClass = "operational"
	DataClassUserContent        DataClass = "user_content"
	DataClassProviderPayload    DataClass = "provider_payload"
	DataClassArtifactMetadata   DataClass = "artifact_metadata"
	DataClassAnalyticsAggregate DataClass = "analytics_aggregate"
	DataClassTemporaryCache     DataClass = "temporary_cache"
)

// Valid reports whether the data class is one of the known policy classes.
func (c DataClass) Valid() bool {
	_, ok := dataClassPolicies[c]
	return ok
}

// DataRetentionClass is a coarse retention bucket attached to a data class.
type DataRetentionClass string

const (
	DataRetentionProtectedAudit DataRetentionClass = "protected_audit"
	DataRetentionLongLived      DataRetentionClass = "long_lived"
	DataRetentionBounded        DataRetentionClass = "bounded"
	DataRetentionShortLived     DataRetentionClass = "short_lived"
	DataRetentionTTL            DataRetentionClass = "ttl"
)

// Valid reports whether the retention class is one of the known buckets.
func (c DataRetentionClass) Valid() bool {
	switch c {
	case DataRetentionProtectedAudit,
		DataRetentionLongLived,
		DataRetentionBounded,
		DataRetentionShortLived,
		DataRetentionTTL:
		return true
	default:
		return false
	}
}

// ArtifactTier identifies the product retention tier for artifact binaries and
// their variants. It is intentionally independent from provider/model routing.
type ArtifactTier string

const (
	ArtifactTierStandard  ArtifactTier = "standard"
	ArtifactTierFree      ArtifactTier = "free"
	ArtifactTierPaid      ArtifactTier = "paid"
	ArtifactTierTemporary ArtifactTier = "temporary"
)

// Valid reports whether the artifact tier is one of the known bounded values.
func (t ArtifactTier) Valid() bool {
	switch t {
	case ArtifactTierStandard,
		ArtifactTierFree,
		ArtifactTierPaid,
		ArtifactTierTemporary:
		return true
	default:
		return false
	}
}

// DataClassPolicy describes what maintenance code is allowed to do with a class.
type DataClassPolicy struct {
	Class             DataClass
	Retention         DataRetentionClass
	AutoDeleteAllowed bool
	RedactionRequired bool
	ContainsPII       bool
	Description       string
}

var dataClassPolicies = map[DataClass]DataClassPolicy{
	DataClassFinancial: {
		Class:             DataClassFinancial,
		Retention:         DataRetentionProtectedAudit,
		AutoDeleteAllowed: false,
		RedactionRequired: false,
		ContainsPII:       true,
		Description:       "ledger, balances, payment intents/events/refunds and audit-critical billing records",
	},
	DataClassOperational: {
		Class:             DataClassOperational,
		Retention:         DataRetentionLongLived,
		AutoDeleteAllowed: true,
		RedactionRequired: false,
		ContainsPII:       false,
		Description:       "jobs metadata, delivery state, provider task ids, idempotency state and support diagnostics",
	},
	DataClassUserContent: {
		Class:             DataClassUserContent,
		Retention:         DataRetentionBounded,
		AutoDeleteAllowed: true,
		RedactionRequired: true,
		ContainsPII:       true,
		Description:       "prompts, conversation messages, assistant replies and user-provided generation inputs",
	},
	DataClassProviderPayload: {
		Class:             DataClassProviderPayload,
		Retention:         DataRetentionShortLived,
		AutoDeleteAllowed: true,
		RedactionRequired: true,
		ContainsPII:       true,
		Description:       "raw or semi-raw provider request/response payloads and provider-native debug data",
	},
	DataClassArtifactMetadata: {
		Class:             DataClassArtifactMetadata,
		Retention:         DataRetentionLongLived,
		AutoDeleteAllowed: true,
		RedactionRequired: false,
		ContainsPII:       false,
		Description:       "artifact ownership, lifecycle, storage coordinates and moderation-safe metadata",
	},
	DataClassAnalyticsAggregate: {
		Class:             DataClassAnalyticsAggregate,
		Retention:         DataRetentionLongLived,
		AutoDeleteAllowed: false,
		RedactionRequired: false,
		ContainsPII:       false,
		Description:       "bounded-label aggregate metrics such as DAU, jobs by model, payment funnel and provider errors",
	},
	DataClassTemporaryCache: {
		Class:             DataClassTemporaryCache,
		Retention:         DataRetentionTTL,
		AutoDeleteAllowed: true,
		RedactionRequired: false,
		ContainsPII:       false,
		Description:       "Redis cache, cooldowns, locks, short-lived menu/dialog state and runtime counters",
	},
}

// Policy returns the retention/security policy for this data class.
func (c DataClass) Policy() (DataClassPolicy, bool) {
	policy, ok := dataClassPolicies[c]
	return policy, ok
}

// MustPolicy returns the retention/security policy for this data class and
// fails closed for unknown values.
func (c DataClass) MustPolicy() DataClassPolicy {
	if policy, ok := c.Policy(); ok {
		return policy
	}
	return DataClassPolicy{
		Class:             c,
		Retention:         DataRetentionProtectedAudit,
		AutoDeleteAllowed: false,
		RedactionRequired: true,
		ContainsPII:       true,
		Description:       "unknown data class; treat as sensitive protected data until classified",
	}
}

// AutoDeleteAllowed reports whether generic maintenance is allowed to delete
// this data class. Financial/audit data is intentionally protected.
func (c DataClass) AutoDeleteAllowed() bool {
	return c.MustPolicy().AutoDeleteAllowed
}

// RequiresRedaction reports whether values in this class must be redacted
// before long-term storage, diagnostics or operator-facing views.
func (c DataClass) RequiresRedaction() bool {
	return c.MustPolicy().RedactionRequired
}

// DataClassPolicies returns a stable list of all known data class policies.
func DataClassPolicies() []DataClassPolicy {
	classes := []DataClass{
		DataClassFinancial,
		DataClassOperational,
		DataClassUserContent,
		DataClassProviderPayload,
		DataClassArtifactMetadata,
		DataClassAnalyticsAggregate,
		DataClassTemporaryCache,
	}
	policies := make([]DataClassPolicy, 0, len(classes))
	for _, class := range classes {
		policies = append(policies, dataClassPolicies[class])
	}
	return policies
}
