package admin

import (
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// pagination is the echoed paging metadata for list responses.
type pagination struct {
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
	Count      int    `json:"count"`
	HasMore    bool   `json:"has_more"`
	Cursor     string `json:"cursor,omitempty"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// listResponse is the envelope for paginated list endpoints.
type listResponse[T any] struct {
	Items      []T        `json:"items"`
	Pagination pagination `json:"pagination"`
}

// OverviewDTO is a bounded, secret-free operational summary for the first
// read-only operator dashboard.
type OverviewDTO struct {
	GeneratedAt time.Time         `json:"generated_at"`
	Cards       []OverviewCardDTO `json:"cards"`
}

// OperatorAccessDTO is the safe role/permission contract for the operator UI.
// It deliberately reports names and boundaries only, never tokens or identities.
type OperatorAccessDTO struct {
	GeneratedAt      time.Time               `json:"generated_at"`
	CurrentAuthMode  string                  `json:"current_auth_mode"`
	EffectiveRole    string                  `json:"effective_role"`
	GlobalBoundaries []string                `json:"global_boundaries"`
	Roles            []OperatorRoleAccessDTO `json:"roles"`
	Notes            []string                `json:"notes,omitempty"`
}

// OperatorRoleAccessDTO is one safe role row for UI capability gating.
type OperatorRoleAccessDTO struct {
	Role           string   `json:"role"`
	Description    string   `json:"description"`
	Permissions    []string `json:"permissions"`
	DataBoundaries []string `json:"data_boundaries"`
	Forbidden      []string `json:"forbidden"`
}

// OverviewCardDTO summarizes one product area without raw entity identifiers,
// payment/provider payloads or private URLs.
type OverviewCardDTO struct {
	ID      string              `json:"id"`
	Title   string              `json:"title"`
	Status  string              `json:"status"`
	Summary string              `json:"summary"`
	Metrics []OverviewMetricDTO `json:"metrics,omitempty"`
}

// OverviewMetricDTO contains bounded display metrics only.
type OverviewMetricDTO struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Status string `json:"status,omitempty"`
}

// OperatorJobsDTO is the safe read-only Jobs screen payload. LookupID is an
// opaque protected-admin lookup value used by the UI to request details; display
// should prefer DisplayID and safe refs.
type OperatorJobsDTO struct {
	GeneratedAt time.Time               `json:"generated_at"`
	Items       []OperatorJobListItem   `json:"items"`
	Pagination  pagination              `json:"pagination"`
	Queue       OperatorQueueSummaryDTO `json:"queue"`
}

// OperatorJobListItem is a bounded job row without raw user/VK identifiers,
// prompts, params, provider payloads or private artifact URLs.
type OperatorJobListItem struct {
	LookupID       string    `json:"lookup_id"`
	DisplayID      string    `json:"display_id"`
	CorrelationRef string    `json:"correlation_ref,omitempty"`
	Operation      string    `json:"operation"`
	Modality       string    `json:"modality"`
	Status         string    `json:"status"`
	ErrorClass     string    `json:"error_class,omitempty"`
	CostEstimate   int64     `json:"cost_estimate"`
	CostReserved   int64     `json:"cost_reserved"`
	CostCaptured   int64     `json:"cost_captured"`
	InputCount     int       `json:"input_count"`
	OutputCount    int       `json:"output_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	AgeSeconds     int64     `json:"age_seconds"`
}

// OperatorJobDetailDTO is a safe job detail view for operator triage.
type OperatorJobDetailDTO struct {
	Job            OperatorJobListItem       `json:"job"`
	AllowedNext    []string                  `json:"allowed_next_statuses"`
	Artifacts      OperatorJobArtifactsDTO   `json:"artifacts"`
	Reservation    *OperatorReservationDTO   `json:"reservation,omitempty"`
	Delivery       OperatorDeliverySummary   `json:"delivery"`
	DeliveryEvents []OperatorDeliveryAttempt `json:"delivery_events"`
}

// OperatorJobArtifactsDTO exposes safe artifact references only.
type OperatorJobArtifactsDTO struct {
	InputRefs  []string `json:"input_refs"`
	OutputRefs []string `json:"output_refs"`
}

// OperatorReservationDTO shows ledger-backed reservation state without account
// ids or idempotency keys.
type OperatorReservationDTO struct {
	Status    string    `json:"status"`
	Amount    int64     `json:"amount"`
	ExpiresAt time.Time `json:"expires_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OperatorDeliverySummary summarizes persisted delivery attempts.
type OperatorDeliverySummary struct {
	Status             string `json:"status"`
	Attempts           int    `json:"attempts"`
	RetryCount         int    `json:"retry_count"`
	LastErrorClass     string `json:"last_error_class,omitempty"`
	LastArtifactRef    string `json:"last_artifact_ref,omitempty"`
	LastDeliveryType   string `json:"last_delivery_type,omitempty"`
	LastDeliveryStatus string `json:"last_delivery_status,omitempty"`
}

// OperatorDeliveryAttempt is a safe delivery attempt row.
type OperatorDeliveryAttempt struct {
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	AttemptNo   int       `json:"attempt_no"`
	ErrorClass  string    `json:"error_class,omitempty"`
	ArtifactRef string    `json:"artifact_ref,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// OperatorQueueSummaryDTO is a safe worker/queue pressure snapshot. It is
// derived from persisted job state and exposes only bounded counters.
type OperatorQueueSummaryDTO struct {
	GeneratedAt            time.Time                `json:"generated_at"`
	DegradationState       string                   `json:"degradation_state"`
	Backlog                []OperatorQueueMetricDTO `json:"backlog"`
	OldestQueuedAgeSeconds *int64                   `json:"oldest_queued_age_seconds,omitempty"`
	RetryCount             int                      `json:"retry_count"`
	DLQ                    OperatorDLQSummaryDTO    `json:"dlq"`
	ProviderCircuit        OperatorQueueNotWiredDTO `json:"provider_circuit"`
	Notes                  []string                 `json:"notes,omitempty"`
}

// OperatorQueueMetricDTO is a bounded queue metric row.
type OperatorQueueMetricDTO struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Status string `json:"status"`
}

// OperatorQueueNotWiredDTO marks data that needs a dedicated source before the
// UI can render it as healthy.
type OperatorQueueNotWiredDTO struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// OperatorDLQSummaryDTO is a bounded DLQ snapshot for queue overview cards.
type OperatorDLQSummaryDTO struct {
	Status           string `json:"status"`
	Reason           string `json:"reason"`
	RetryableCount   int    `json:"retryable_count"`
	TerminalCount    int    `json:"terminal_count"`
	BatchReplayLimit int    `json:"batch_replay_limit"`
}

// OperatorDLQDTO is the safe DLQ console payload. It must not expose prompts,
// provider payloads, private URLs or user identifiers.
type OperatorDLQDTO struct {
	GeneratedAt time.Time                  `json:"generated_at"`
	Items       []OperatorDLQItemDTO       `json:"items"`
	Pagination  pagination                 `json:"pagination"`
	Replay      OperatorDLQReplayPolicyDTO `json:"replay"`
	Notes       []string                   `json:"notes,omitempty"`
}

// OperatorDLQItemDTO is one bounded DLQ row with replay guard-rail metadata.
type OperatorDLQItemDTO struct {
	Job                 OperatorJobListItem `json:"job"`
	AttemptCount        int                 `json:"attempt_count"`
	ProviderTaskCount   int                 `json:"provider_task_count"`
	LastErrorClass      string              `json:"last_error_class,omitempty"`
	LastProviderClass   string              `json:"last_provider_class,omitempty"`
	SafeReplay          bool                `json:"safe_replay"`
	ReplayBlockedReason string              `json:"replay_blocked_reason,omitempty"`
	ReplayTarget        string              `json:"replay_target"`
}

// OperatorDLQReplayPolicyDTO describes backend-enforced replay boundaries.
type OperatorDLQReplayPolicyDTO struct {
	SingleAllowedStatuses  []string `json:"single_allowed_statuses"`
	BatchLimit             int      `json:"batch_limit"`
	BatchSkipsPaidProvider bool     `json:"batch_skips_paid_provider"`
	Notes                  []string `json:"notes,omitempty"`
}

// OperatorDLQReplayRequestDTO is accepted by replay endpoints. Values are
// guard-rail hints only; backend policy remains authoritative.
type OperatorDLQReplayRequestDTO struct {
	JobIDs            []string `json:"job_ids,omitempty"`
	Limit             int      `json:"limit,omitempty"`
	ErrorClass        string   `json:"error_class,omitempty"`
	AllowPaidProvider bool     `json:"allow_paid_provider,omitempty"`
}

// OperatorDLQReplayResultDTO reports replay/skipped decisions without raw job
// payloads or provider details.
type OperatorDLQReplayResultDTO struct {
	GeneratedAt time.Time                  `json:"generated_at"`
	Requested   int                        `json:"requested"`
	Replayed    []OperatorDLQReplayItemDTO `json:"replayed"`
	Skipped     []OperatorDLQReplayItemDTO `json:"skipped"`
	BatchLimit  int                        `json:"batch_limit"`
}

// OperatorDLQReplayItemDTO is one replay decision row.
type OperatorDLQReplayItemDTO struct {
	LookupID  string `json:"lookup_id"`
	DisplayID string `json:"display_id"`
	Status    string `json:"status"`
	Result    string `json:"result"`
	Reason    string `json:"reason,omitempty"`
}

// OperatorProviderControlRoomDTO is a bounded provider/media business-risk view.
// It uses curated provider/model classes only and never exposes raw model IDs,
// provider payloads, prompts, private URLs or user identities.
type OperatorProviderControlRoomDTO struct {
	GeneratedAt        time.Time                   `json:"generated_at"`
	Providers          []OperatorProviderHealthDTO `json:"providers"`
	VideoRoutes        []OperatorVideoRouteDTO     `json:"video_routes"`
	Fallback           OperatorProviderFallbackDTO `json:"fallback"`
	ProviderWaste      OperatorRiskSignalDTO       `json:"provider_waste"`
	DeliveryCaptureGap OperatorRiskSignalDTO       `json:"delivery_capture_gap"`
	Circuit            OperatorNotWiredSignalDTO   `json:"circuit"`
	Notes              []string                    `json:"notes,omitempty"`
}

// OperatorProviderHealthDTO summarizes one curated provider/model class.
type OperatorProviderHealthDTO struct {
	ProviderClass       string     `json:"provider_class"`
	ServiceType         string     `json:"service_type"`
	ModelClass          string     `json:"model_class"`
	Modality            string     `json:"modality"`
	Health              string     `json:"health"`
	CircuitState        string     `json:"circuit_state"`
	QuotaState          string     `json:"quota_state"`
	CooldownState       string     `json:"cooldown_state"`
	RateLimitCount      int        `json:"rate_limit_count"`
	ProviderFailedCount int        `json:"provider_failed_count"`
	InvalidOutputCount  int        `json:"invalid_output_count"`
	ObservedTotalCount  int64      `json:"observed_total_count"`
	ErrorRatePercent    float64    `json:"error_rate_percent"`
	LatencyP95Ms        int64      `json:"latency_p95_ms"`
	InFlightCount       int64      `json:"in_flight_count"`
	LatestErrorClass    string     `json:"latest_error_class,omitempty"`
	LatestErrorAt       *time.Time `json:"latest_error_at,omitempty"`
	FallbackState       string     `json:"fallback_state"`
	ContractConfigured  bool       `json:"contract_configured"`
	QualityGuardEnabled bool       `json:"quality_guard_enabled"`
	Source              string     `json:"source"`
}

// OperatorProviderFallbackDTO reports whether fallback is configured without
// exposing provider-native routing internals.
type OperatorProviderFallbackDTO struct {
	Status          string   `json:"status"`
	ProviderClasses []string `json:"provider_classes"`
	Summary         string   `json:"summary"`
}

// OperatorRiskSignalDTO is a generic bounded risk counter for operator triage.
type OperatorRiskSignalDTO struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Value   string `json:"value"`
	Source  string `json:"source"`
	Summary string `json:"summary"`
}

// OperatorNotWiredSignalDTO marks a missing dedicated source explicitly.
type OperatorNotWiredSignalDTO struct {
	Status  string `json:"status"`
	Source  string `json:"source"`
	Summary string `json:"summary"`
}

// OperatorMediaSafetyDTO is a read-only safe media-pipeline control room.
type OperatorMediaSafetyDTO struct {
	GeneratedAt time.Time               `json:"generated_at"`
	Policy      OperatorMediaPolicyDTO  `json:"policy"`
	Uploads     []OperatorRiskSignalDTO `json:"uploads"`
	Queue       OperatorQueueSummaryDTO `json:"queue"`
	Processing  []OperatorRiskSignalDTO `json:"processing"`
	Cleanup     OperatorRiskSignalDTO   `json:"cleanup"`
	Notes       []string                `json:"notes,omitempty"`
}

// OperatorMediaPolicyDTO exposes non-secret media flags and limits only.
type OperatorMediaPolicyDTO struct {
	PipelineEnabled                 bool   `json:"pipeline_enabled"`
	ProbePolicy                     string `json:"probe_policy"`
	TranscodePolicy                 string `json:"transcode_policy"`
	RawProviderVideoPolicy          string `json:"raw_provider_video_policy"`
	ReferenceUploadsEnabled         bool   `json:"reference_uploads_enabled"`
	WebPReferenceEnabled            bool   `json:"webp_reference_enabled"`
	MaxImageUploadBytes             int64  `json:"max_image_upload_bytes"`
	MaxImagePixels                  int64  `json:"max_image_pixels"`
	MaxVideoSizeBytes               int64  `json:"max_video_size_bytes"`
	MaxVideoDurationSec             int    `json:"max_video_duration_sec"`
	MaxConcurrentUploads            int    `json:"max_concurrent_uploads"`
	MaxConcurrentProbes             int    `json:"max_concurrent_probes"`
	MaxConcurrentTranscodes         int    `json:"max_concurrent_transcodes"`
	MaxPendingVariants              int    `json:"max_pending_variants"`
	QueueDegradeThreshold           int64  `json:"queue_degrade_threshold"`
	ProviderMaxAttemptsPerJob       int    `json:"provider_max_attempts_per_job"`
	ProviderFallbackBudgetPerJob    int    `json:"provider_fallback_budget_per_job"`
	ProviderQualityGuardEnabled     bool   `json:"provider_quality_guard_enabled"`
	ProviderQualityDegradedFailures int    `json:"provider_quality_degraded_failures"`
	ProviderQualityDisabledFailures int    `json:"provider_quality_disabled_failures"`
}

// OperatorConfigHealthDTO reports non-secret runtime posture. It deliberately
// omits secrets, raw model IDs, local binary paths and URLs.
type OperatorConfigHealthDTO struct {
	GeneratedAt     time.Time                    `json:"generated_at"`
	Environment     string                       `json:"environment"`
	Flags           []OperatorConfigFlagDTO      `json:"flags"`
	ProviderClasses []OperatorRuntimeProviderDTO `json:"provider_classes"`
	VideoRoutes     []OperatorVideoRouteDTO      `json:"video_routes"`
	Notes           []string                     `json:"notes,omitempty"`
}

// OperatorConfigFlagDTO is a single non-secret config flag.
type OperatorConfigFlagDTO struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

// OperatorRetentionStatusDTO is a safe retention control-room response. It
// exposes counts and ages only; no raw prompts, payloads, ids or storage URLs.
type OperatorRetentionStatusDTO struct {
	GeneratedAt     time.Time                   `json:"generated_at"`
	Retention       []OperatorRetentionTableDTO `json:"retention"`
	OldestHotRows   []OperatorOldestHotRowDTO   `json:"oldest_hot_rows"`
	OrphanArtifacts OperatorOrphanArtifactsDTO  `json:"orphan_artifacts"`
	Notes           []string                    `json:"notes,omitempty"`
}

// OperatorRetentionTableDTO summarizes one retention table/class pair.
type OperatorRetentionTableDTO struct {
	TableName           string     `json:"table_name"`
	RetentionClass      string     `json:"retention_class"`
	TotalRows           int64      `json:"total_rows"`
	ExpiredRows         int64      `json:"expired_rows"`
	RedactedRows        int64      `json:"redacted_rows"`
	DeletedRows         int64      `json:"deleted_rows"`
	OldestHotAt         *time.Time `json:"oldest_hot_at,omitempty"`
	OldestHotAgeSeconds int64      `json:"oldest_hot_age_seconds"`
	OldestExpiredAt     *time.Time `json:"oldest_expired_at,omitempty"`
}

// OperatorRetentionDryRunDTO reports cleanup candidates without mutating data.
type OperatorRetentionDryRunDTO struct {
	GeneratedAt time.Time                        `json:"generated_at"`
	Items       []OperatorRetentionDryRunDTOItem `json:"items"`
	Notes       []string                         `json:"notes,omitempty"`
}

// OperatorRetentionCleanupDTO reports the safe post-cleanup retention snapshot.
// It does not include raw deleted rows, prompts, payloads, storage paths or
// financial table mutations.
type OperatorRetentionCleanupDTO struct {
	GeneratedAt     time.Time                   `json:"generated_at"`
	Completed       bool                        `json:"completed"`
	Retention       []OperatorRetentionTableDTO `json:"retention"`
	OldestHotRows   []OperatorOldestHotRowDTO   `json:"oldest_hot_rows"`
	OrphanArtifacts OperatorOrphanArtifactsDTO  `json:"orphan_artifacts"`
	Notes           []string                    `json:"notes,omitempty"`
}

// OperatorRetentionDryRunDTOItem is one dry-run action row.
type OperatorRetentionDryRunDTOItem struct {
	Action           string     `json:"action"`
	TableName        string     `json:"table_name"`
	RetentionClass   string     `json:"retention_class"`
	Count            int64      `json:"count"`
	Bytes            int64      `json:"bytes"`
	OldestAt         *time.Time `json:"oldest_at,omitempty"`
	OldestAgeSeconds int64      `json:"oldest_age_seconds"`
}

// OperatorAnalyticsStatusDTO reports aggregate table freshness.
type OperatorAnalyticsStatusDTO struct {
	GeneratedAt time.Time                        `json:"generated_at"`
	Items       []OperatorAnalyticsStatusItemDTO `json:"items"`
	Notes       []string                         `json:"notes,omitempty"`
}

// OperatorAnalyticsStatusItemDTO is one aggregate table status row.
type OperatorAnalyticsStatusItemDTO struct {
	TableName             string     `json:"table_name"`
	Status                string     `json:"status"`
	Rows                  int64      `json:"rows"`
	LatestActivityDate    *time.Time `json:"latest_activity_date,omitempty"`
	LastUpdatedAt         *time.Time `json:"last_updated_at,omitempty"`
	LastUpdatedAgeSeconds int64      `json:"last_updated_age_seconds"`
}

// OperatorOldestHotRowsDTO reports oldest hot rows by table/class.
type OperatorOldestHotRowsDTO struct {
	GeneratedAt time.Time                 `json:"generated_at"`
	Items       []OperatorOldestHotRowDTO `json:"items"`
	Notes       []string                  `json:"notes,omitempty"`
}

// OperatorOldestHotRowDTO is a bounded oldest-row signal.
type OperatorOldestHotRowDTO struct {
	TableName      string     `json:"table_name"`
	RetentionClass string     `json:"retention_class"`
	Count          int64      `json:"count"`
	OldestAt       *time.Time `json:"oldest_at,omitempty"`
	AgeSeconds     int64      `json:"age_seconds"`
}

// OperatorOrphanArtifactsDTO groups orphan artifact candidates without ids,
// owners, buckets, keys or private URLs.
type OperatorOrphanArtifactsDTO struct {
	GeneratedAt time.Time                   `json:"generated_at"`
	Total       int64                       `json:"total"`
	Bytes       int64                       `json:"bytes"`
	Items       []OperatorOrphanArtifactDTO `json:"items"`
	Notes       []string                    `json:"notes,omitempty"`
}

// OperatorOrphanArtifactDTO is one safe orphan artifact group.
type OperatorOrphanArtifactDTO struct {
	ArtifactTier     string     `json:"artifact_tier"`
	LifecycleClass   string     `json:"lifecycle_class"`
	Status           string     `json:"status"`
	MediaType        string     `json:"media_type"`
	Count            int64      `json:"count"`
	Bytes            int64      `json:"bytes"`
	OldestAt         *time.Time `json:"oldest_at,omitempty"`
	OldestAgeSeconds int64      `json:"oldest_age_seconds"`
}

// OperatorRuntimeProviderDTO is a curated provider/model class row.
type OperatorRuntimeProviderDTO struct {
	ProviderClass      string `json:"provider_class"`
	ModelClass         string `json:"model_class"`
	Modality           string `json:"modality"`
	ContractConfigured bool   `json:"contract_configured"`
}

// OperatorVideoRouteDTO is the safe admin view of a public video route. It
// intentionally omits provider-native model IDs, URLs, pricing and secrets.
type OperatorVideoRouteDTO struct {
	Alias                  string   `json:"alias"`
	ProviderClass          string   `json:"provider_class"`
	ModelClass             string   `json:"model_class"`
	Status                 string   `json:"status"`
	Reason                 string   `json:"reason"`
	Enabled                bool     `json:"enabled"`
	ProviderEnabled        bool     `json:"provider_enabled"`
	ProviderConfigured     bool     `json:"provider_configured"`
	ProviderBaseConfigured bool     `json:"provider_base_configured"`
	CostConfigured         bool     `json:"cost_configured"`
	RequiresStartImage     bool     `json:"requires_start_image"`
	SupportsReferenceImage bool     `json:"supports_reference_image"`
	MaxReferenceImages     int      `json:"max_reference_images,omitempty"`
	AllowedDurationsSec    []int    `json:"allowed_durations_sec,omitempty"`
	AllowedResolutions     []string `json:"allowed_resolutions,omitempty"`
}

// OperatorPricingDTO is the read-only runtime generation pricing view for
// operators. It lists public product dimensions and backend-calculated credit
// estimates only; floor, multiplier, provider cost, provider-native ids,
// prompts, payloads and private URLs are intentionally omitted.
type OperatorPricingDTO struct {
	GeneratedAt    time.Time                 `json:"generated_at"`
	Source         string                    `json:"source"`
	Version        int                       `json:"version"`
	StaticFallback bool                      `json:"static_fallback"`
	LoadedAt       time.Time                 `json:"loaded_at"`
	EffectiveFrom  *time.Time                `json:"effective_from,omitempty"`
	EffectiveUntil *time.Time                `json:"effective_until,omitempty"`
	Entries        []OperatorPricingEntryDTO `json:"entries"`
	EntryCount     int                       `json:"entry_count"`
	Notes          []string                  `json:"notes,omitempty"`
}

// OperatorPricingEntryDTO is one public product key with pricingcatalog-derived
// display and exact estimates. It does not include provider routing data.
type OperatorPricingEntryDTO struct {
	Operation              string `json:"operation"`
	Modality               string `json:"modality"`
	ImageModelID           string `json:"image_model_id,omitempty"`
	VideoRouteAlias        string `json:"video_route_alias,omitempty"`
	Quality                string `json:"quality,omitempty"`
	Resolution             string `json:"resolution,omitempty"`
	DurationSec            int    `json:"duration_sec,omitempty"`
	CostEstimateCredits    int64  `json:"cost_estimate_credits"`
	DisplayEstimateCredits int64  `json:"display_estimate_credits"`
	Enabled                bool   `json:"enabled"`
}

// OperatorUsersDTO is the read-only safe user console payload. It never exposes
// raw VK user ids, names, timezones or internal UUIDs by default.
type OperatorUsersDTO struct {
	GeneratedAt time.Time                      `json:"generated_at"`
	User        *OperatorUserSummaryDTO        `json:"user,omitempty"`
	RecentJobs  []OperatorUserRecentJobDTO     `json:"recent_jobs,omitempty"`
	Payment     OperatorUserPaymentSummaryDTO  `json:"payment"`
	Referrals   OperatorUserReferralSummaryDTO `json:"referrals"`
	Notes       []string                       `json:"notes,omitempty"`
}

type OperatorUserSummaryDTO struct {
	UserRef     string                    `json:"user_ref"`
	Role        string                    `json:"role"`
	Status      string                    `json:"status"`
	Locale      string                    `json:"locale,omitempty"`
	RiskClass   string                    `json:"risk_class"`
	FirstSeenAt *time.Time                `json:"first_seen_at,omitempty"`
	LastSeenAt  *time.Time                `json:"last_seen_at,omitempty"`
	CreatedAt   time.Time                 `json:"created_at"`
	UpdatedAt   time.Time                 `json:"updated_at"`
	AgeSeconds  int64                     `json:"age_seconds"`
	Jobs        OperatorUserJobSummaryDTO `json:"jobs"`
}

type OperatorUserJobSummaryDTO struct {
	Status         string `json:"status"`
	Total          string `json:"total"`
	Active         string `json:"active"`
	Succeeded      string `json:"succeeded"`
	Failed         string `json:"failed"`
	TextJobs       string `json:"text_jobs"`
	ImageJobs      string `json:"image_jobs"`
	VideoJobs      string `json:"video_jobs"`
	RecentPageSize int    `json:"recent_page_size"`
}

type OperatorUserRecentJobDTO struct {
	DisplayID    string    `json:"display_id"`
	Operation    string    `json:"operation"`
	Modality     string    `json:"modality"`
	Status       string    `json:"status"`
	ErrorClass   string    `json:"error_class,omitempty"`
	CostReserved int64     `json:"cost_reserved"`
	CostCaptured int64     `json:"cost_captured"`
	CreatedAt    time.Time `json:"created_at"`
	AgeSeconds   int64     `json:"age_seconds"`
}

type OperatorUserPaymentSummaryDTO struct {
	Status           string `json:"status"`
	Total            int    `json:"total"`
	Pending          int    `json:"pending"`
	Succeeded        int    `json:"succeeded"`
	Failed           int    `json:"failed"`
	Refunded         int    `json:"refunded"`
	CreditsPurchased int64  `json:"credits_purchased"`
}

type OperatorUserReferralSummaryDTO struct {
	Status     string                    `json:"status"`
	Code       string                    `json:"code,omitempty"`
	Invited    int                       `json:"invited"`
	Registered int                       `json:"registered"`
	Activated  int                       `json:"activated"`
	Rewarded   int                       `json:"rewarded"`
	InvitedBy  *OperatorUserInvitedByDTO `json:"invited_by,omitempty"`
}

type OperatorUserInvitedByDTO struct {
	Source       string `json:"source"`
	Status       string `json:"status"`
	RewardStatus string `json:"reward_status"`
}

// OperatorReferralsDTO is a no-PII referral console payload.
type OperatorReferralsDTO struct {
	GeneratedAt        time.Time                             `json:"generated_at"`
	CodeStats          *ReferralStatsDTO                     `json:"code_stats,omitempty"`
	Distribution       OperatorReferralDistributionDTO       `json:"distribution"`
	Suspicious         []SuspiciousReferralDTO               `json:"suspicious"`
	SuspiciousCriteria OperatorReferralSuspiciousCriteriaDTO `json:"suspicious_criteria"`
	Pagination         pagination                            `json:"pagination"`
	Notes              []string                              `json:"notes,omitempty"`
}

type OperatorReferralDistributionDTO struct {
	RegisteredCount int `json:"registered_count"`
	ActivatedCount  int `json:"activated_count"`
	RewardedCount   int `json:"rewarded_count"`
	Total           int `json:"total"`
}

type OperatorReferralSuspiciousCriteriaDTO struct {
	MinRegistered int `json:"min_registered"`
	MinTotal      int `json:"min_total"`
}

// OperatorAuditLogDTO is a sanitized read-only operator audit listing.
type OperatorAuditLogDTO struct {
	GeneratedAt time.Time               `json:"generated_at"`
	Items       []OperatorAuditEntryDTO `json:"items"`
	Pagination  pagination              `json:"pagination"`
	Notes       []string                `json:"notes,omitempty"`
}

type OperatorAuditEntryDTO struct {
	DisplayID  string    `json:"display_id"`
	ActorRef   string    `json:"actor_ref"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetRef  string    `json:"target_ref,omitempty"`
	Result     string    `json:"result"`
	RequestRef string    `json:"request_ref,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// JobDTO is the admin representation of a job.
type JobDTO struct {
	ID                uuid.UUID   `json:"id"`
	UserID            uuid.UUID   `json:"user_id"`
	VKPeerID          int64       `json:"vk_peer_id"`
	Operation         string      `json:"operation"`
	Modality          string      `json:"modality"`
	Status            string      `json:"status"`
	CostEstimate      int64       `json:"cost_estimate"`
	CostReserved      int64       `json:"cost_reserved"`
	CostCaptured      int64       `json:"cost_captured"`
	OutputArtifactIDs []uuid.UUID `json:"output_artifact_ids"`
	ErrorCode         string      `json:"error_code,omitempty"`
	ErrorMessage      string      `json:"error_message,omitempty"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

func newJobDTO(j *domain.Job) JobDTO {
	out := JobDTO{
		ID:                j.ID,
		UserID:            j.UserID,
		VKPeerID:          j.VKPeerID,
		Operation:         string(j.OperationType),
		Modality:          string(j.Modality),
		Status:            string(j.Status),
		CostEstimate:      j.CostEstimate,
		CostReserved:      j.CostReserved,
		CostCaptured:      j.CostCaptured,
		OutputArtifactIDs: j.OutputArtifactIDs,
		ErrorCode:         j.ErrorCode,
		ErrorMessage:      j.ErrorMessage,
		CreatedAt:         j.CreatedAt,
		UpdatedAt:         j.UpdatedAt,
	}
	if out.OutputArtifactIDs == nil {
		out.OutputArtifactIDs = []uuid.UUID{}
	}
	return out
}

// UserDTO is the admin representation of a user.
type UserDTO struct {
	ID             uuid.UUID `json:"id"`
	VKUserID       int64     `json:"vk_user_id"`
	Role           string    `json:"role"`
	Status         string    `json:"status"`
	Locale         string    `json:"locale"`
	BalanceCredits *int64    `json:"balance_credits,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func newUserDTO(u *domain.User) UserDTO {
	return UserDTO{
		ID:        u.ID,
		VKUserID:  u.VKUserID,
		Role:      string(u.Role),
		Status:    string(u.Status),
		Locale:    u.Locale,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

// DeliveryDTO is the admin representation of a delivery attempt.
type DeliveryDTO struct {
	ID           uuid.UUID  `json:"id"`
	JobID        uuid.UUID  `json:"job_id"`
	UserID       uuid.UUID  `json:"user_id"`
	VKPeerID     int64      `json:"vk_peer_id"`
	ArtifactID   *uuid.UUID `json:"artifact_id,omitempty"`
	Type         string     `json:"type"`
	Status       string     `json:"status"`
	VKRandomID   int64      `json:"vk_random_id"`
	VKMessageID  *int64     `json:"vk_message_id,omitempty"`
	Attachment   string     `json:"attachment,omitempty"`
	AttemptNo    int        `json:"attempt_no"`
	ErrorCode    string     `json:"error_code,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func newDeliveryDTO(d *domain.Delivery) DeliveryDTO {
	return DeliveryDTO{
		ID:           d.ID,
		JobID:        d.JobID,
		UserID:       d.UserID,
		VKPeerID:     d.VKPeerID,
		ArtifactID:   d.ArtifactID,
		Type:         string(d.Type),
		Status:       string(d.Status),
		VKRandomID:   d.VKRandomID,
		VKMessageID:  d.VKMessageID,
		Attachment:   d.Attachment,
		AttemptNo:    d.AttemptNo,
		ErrorCode:    d.ErrorCode,
		ErrorMessage: d.ErrorMessage,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
}

// ReferralStatsDTO is a safe operator view of one public referral code.
type ReferralStatsDTO struct {
	Code            string `json:"code"`
	InvitedCount    int    `json:"invited_count"`
	RegisteredCount int    `json:"registered_count"`
	ActivatedCount  int    `json:"activated_count"`
	RewardedCount   int    `json:"rewarded_count"`
}

func newReferralStatsDTO(stats domain.ReferralCodeStats) ReferralStatsDTO {
	return ReferralStatsDTO{
		Code:            stats.Code,
		InvitedCount:    stats.Stats.Total(),
		RegisteredCount: stats.Stats.RegisteredCount,
		ActivatedCount:  stats.Stats.ActivatedCount,
		RewardedCount:   stats.Stats.RewardedCount,
	}
}

// SuspiciousReferralDTO explains aggregate referral-code patterns without
// exposing invited-user identities.
type SuspiciousReferralDTO struct {
	ReferralStatsDTO
	Reasons []string `json:"reasons"`
}

type referralFutureFlagDTO struct {
	Code    string `json:"code"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
	Message string `json:"message"`
}
