package domain

import (
	"time"

	"github.com/google/uuid"
)

// BalanceMismatch reports a cached account balance that differs from the
// append-only committed ledger projection.
type BalanceMismatch struct {
	AccountID     uuid.UUID `json:"account_id"`
	UserID        uuid.UUID `json:"user_id"`
	Currency      Currency  `json:"currency"`
	BalanceCached int64     `json:"balance_cached"`
	LedgerBalance int64     `json:"ledger_balance"`
	Difference    int64     `json:"difference"`
}

// RetentionStatus is a safe operator read model for retention posture. It
// contains only table/class counters and timestamps, never raw prompts,
// provider payloads, user ids or storage paths.
type RetentionStatus struct {
	GeneratedAt time.Time
	Items       []RetentionStatusItem
}

// RetentionStatusItem summarizes one retention-controlled table/class pair.
type RetentionStatusItem struct {
	TableName       string
	RetentionClass  DataClass
	TotalRows       int64
	ExpiredRows     int64
	RedactedRows    int64
	DeletedRows     int64
	OldestHotAt     *time.Time
	OldestExpiredAt *time.Time
}

// RetentionDryRun reports read-only cleanup candidates. Counts are grouped by
// safe bounded labels only.
type RetentionDryRun struct {
	GeneratedAt time.Time
	Items       []RetentionDryRunItem
}

// RetentionDryRunItem is one dry-run cleanup action candidate.
type RetentionDryRunItem struct {
	Action         string
	TableName      string
	RetentionClass DataClass
	Count          int64
	Bytes          int64
	OldestAt       *time.Time
}

// AnalyticsAggregationStatus reports whether aggregate tables are populated
// without reading raw jobs/messages/payment payloads.
type AnalyticsAggregationStatus struct {
	GeneratedAt time.Time
	Items       []AnalyticsAggregationStatusItem
}

// AnalyticsAggregationStatusItem is a bounded aggregate-table health row.
type AnalyticsAggregationStatusItem struct {
	TableName          string
	Status             string
	Rows               int64
	LatestActivityDate *time.Time
	LastUpdatedAt      *time.Time
}

// OldestHotRowsReport reports oldest still-hot data by table/class. It is
// intended for retention triage and intentionally omits entity identifiers.
type OldestHotRowsReport struct {
	GeneratedAt time.Time
	Items       []OldestHotRow
}

// OldestHotRow is a safe oldest-row summary without PII or raw content.
type OldestHotRow struct {
	TableName      string
	RetentionClass DataClass
	Count          int64
	OldestAt       *time.Time
	AgeSeconds     int64
}

// OrphanArtifactsReport reports artifact metadata/object cleanup candidates
// without exposing private buckets, keys, URLs or owner ids.
type OrphanArtifactsReport struct {
	GeneratedAt time.Time
	Total       int64
	Bytes       int64
	Items       []OrphanArtifactCount
}

// OrphanArtifactCount groups orphan artifact candidates by bounded metadata.
type OrphanArtifactCount struct {
	ArtifactTier   ArtifactTier
	LifecycleClass ArtifactLifecycleClass
	Status         ArtifactStatus
	MediaType      MediaType
	Count          int64
	Bytes          int64
	OldestAt       *time.Time
}
