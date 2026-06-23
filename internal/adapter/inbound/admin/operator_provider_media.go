package admin

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"vk-ai-aggregator/internal/domain"
	platformconfig "vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/videorouter"
)

const (
	operatorSourceJobSnapshot      = "persisted_job_snapshot"
	operatorSourceJobModalityProxy = "persisted_job_modality_proxy"
	operatorSourceRuntimeConfig    = "runtime_config_snapshot"
	operatorSourcePrometheusNeeded = "private_prometheus_source_pending"
)

// RuntimeSnapshot is a sanitized non-secret config projection for operator
// screens. It intentionally omits raw model IDs, URLs, secrets and local paths.
type RuntimeSnapshot struct {
	Environment                          string
	PaymentProvider                      string
	PaymentWebhookHTTPSRequired          bool
	MediaPipelineEnabled                 bool
	MediaVideoProbePolicy                string
	MediaVideoTranscodePolicy            string
	MediaDeliverRawProviderVideo         string
	MediaReferenceUploadsEnabled         bool
	MediaReferenceWebPEnabled            bool
	MediaMaxImageUploadBytes             int64
	MediaMaxImagePixels                  int64
	MediaMaxVideoSizeBytes               int64
	MediaMaxVideoDurationSec             int
	MediaMaxConcurrentUploads            int
	MediaMaxConcurrentProbes             int
	MediaMaxConcurrentTranscodes         int
	MediaMaxPendingVariants              int
	MediaMaxActiveVideoJobsPerUser       int
	MediaProviderMaxAttemptsPerJob       int
	MediaProviderFallbackBudget          int
	MediaQueueDegradeThreshold           int64
	MediaProviderQualityGuardEnabled     bool
	MediaProviderQualityDegradedFailures int
	MediaProviderQualityDisabledFailures int
	ProviderClasses                      []RuntimeProviderClass
	VideoRoutes                          []RuntimeVideoRoute
}

// RuntimeProviderClass is a curated provider/model class. ProviderClass and
// ModelClass are bounded labels, never provider-native model IDs.
type RuntimeProviderClass struct {
	ProviderClass      string
	ModelClass         string
	Modality           string
	ContractConfigured bool
}

// RuntimeVideoRoute is a safe route-readiness projection. It intentionally omits
// provider-native model IDs, URLs, API keys and pricing amounts.
type RuntimeVideoRoute struct {
	Alias                  string
	ProviderClass          string
	ModelClass             string
	Status                 string
	Reason                 string
	Enabled                bool
	ProviderEnabled        bool
	ProviderConfigured     bool
	ProviderBaseConfigured bool
	CostConfigured         bool
	RequiresStartImage     bool
	SupportsReferenceImage bool
	MaxReferenceImages     int
	AllowedDurationsSec    []int
	AllowedResolutions     []string
}

// NewRuntimeSnapshot creates a safe admin projection from the full application
// config. Do not add secrets, URLs, raw model IDs or local binary paths here.
func NewRuntimeSnapshot(cfg platformconfig.Config) RuntimeSnapshot {
	return RuntimeSnapshot{
		Environment:                          safeRuntimeToken(cfg.Env, "unknown"),
		PaymentProvider:                      safeProviderClass(cfg.PaymentProvider),
		PaymentWebhookHTTPSRequired:          cfg.PaymentWebhookHTTPSRequired(),
		MediaPipelineEnabled:                 cfg.MediaPipelineEnabled,
		MediaVideoProbePolicy:                safeRuntimeToken(cfg.EffectiveMediaVideoProbePolicy(), "unknown"),
		MediaVideoTranscodePolicy:            safeRuntimeToken(cfg.EffectiveMediaVideoTranscodePolicy(), "unknown"),
		MediaDeliverRawProviderVideo:         safeRuntimeToken(cfg.EffectiveMediaDeliverRawProviderVideo(), "unknown"),
		MediaReferenceUploadsEnabled:         cfg.MediaReferenceUploadsEnabled,
		MediaReferenceWebPEnabled:            cfg.MediaReferenceWebPEnabled,
		MediaMaxImageUploadBytes:             cfg.MediaMaxImageUploadBytes,
		MediaMaxImagePixels:                  cfg.MediaMaxImagePixels,
		MediaMaxVideoSizeBytes:               cfg.MediaMaxVideoSizeBytes,
		MediaMaxVideoDurationSec:             cfg.MediaMaxVideoDurationSec,
		MediaMaxConcurrentUploads:            cfg.MediaMaxConcurrentUploads,
		MediaMaxConcurrentProbes:             cfg.MediaMaxConcurrentProbes,
		MediaMaxConcurrentTranscodes:         cfg.MediaMaxConcurrentTranscodes,
		MediaMaxPendingVariants:              cfg.MediaMaxPendingVariants,
		MediaMaxActiveVideoJobsPerUser:       cfg.MediaMaxActiveVideoJobsPerUser,
		MediaProviderMaxAttemptsPerJob:       cfg.MediaProviderMaxAttemptsPerJob,
		MediaProviderFallbackBudget:          cfg.MediaProviderFallbackBudget,
		MediaQueueDegradeThreshold:           cfg.MediaQueueDegradeThreshold,
		MediaProviderQualityGuardEnabled:     cfg.MediaProviderQualityGuardEnabled,
		MediaProviderQualityDegradedFailures: cfg.MediaProviderQualityDegradedFailures,
		MediaProviderQualityDisabledFailures: cfg.MediaProviderQualityDisabledFailures,
		ProviderClasses:                      runtimeProviderClasses(cfg),
		VideoRoutes:                          runtimeVideoRoutes(cfg),
	}
}

func (h *Handler) getOperatorProviders(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	classes := h.cfg.Runtime.ProviderClasses
	items := make([]OperatorProviderHealthDTO, 0, len(classes))
	for _, class := range classes {
		items = append(items, h.operatorProviderHealth(r.Context(), class))
	}
	writeJSON(w, http.StatusOK, OperatorProviderControlRoomDTO{
		GeneratedAt:        now,
		Providers:          items,
		VideoRoutes:        h.operatorVideoRouteDTOs(),
		Fallback:           h.operatorProviderFallback(),
		ProviderWaste:      h.operatorProviderWasteSignal(r.Context()),
		DeliveryCaptureGap: h.operatorDeliveryCaptureGapSignal(r.Context()),
		Circuit: OperatorNotWiredSignalDTO{
			Status:  overviewStatusNotWired,
			Source:  operatorSourcePrometheusNeeded,
			Summary: "Live circuit state is worker-owned and not exposed as a bounded admin read model yet.",
		},
		Notes: []string{
			"Counts are bounded read-only snapshots; use private Grafana for live Prometheus time-series.",
			"No provider disable/degrade actions are available in this stage.",
		},
	})
}

func (h *Handler) getOperatorMediaSafety(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	queue := OperatorQueueSummaryDTO{
		GeneratedAt:      now,
		DegradationState: "unknown",
		DLQ: OperatorQueueNotWiredDTO{
			Status: overviewStatusNotWired,
			Reason: "Job repository is not configured for queue pressure summary.",
		},
		ProviderCircuit: OperatorQueueNotWiredDTO{
			Status: overviewStatusNotWired,
			Reason: "Provider circuit state needs a dedicated bounded source.",
		},
	}
	if h.deps.Jobs != nil {
		queue = h.operatorQueueSummary(r.Context(), now)
	}
	writeJSON(w, http.StatusOK, OperatorMediaSafetyDTO{
		GeneratedAt: now,
		Policy:      h.operatorMediaPolicy(),
		Uploads: []OperatorRiskSignalDTO{
			h.operatorUploadRejectSignal(r.Context(), domain.JobErrMediaUploadInvalid, "Invalid image uploads"),
			h.operatorUploadRejectSignal(r.Context(), domain.JobErrMediaUploadTooLarge, "Oversized image uploads"),
			h.operatorUploadRejectSignal(r.Context(), domain.JobErrMediaUploadUnsupported, "Unsupported image uploads"),
		},
		Queue: queue,
		Processing: []OperatorRiskSignalDTO{
			h.operatorInvalidOutputSignal(r.Context()),
			h.operatorFastPathSignal(),
			h.operatorTranscodeFallbackSignal(r.Context()),
		},
		Cleanup: OperatorRiskSignalDTO{
			ID:      "cleanup_stats",
			Title:   "Cleanup stats",
			Status:  overviewStatusNotWired,
			Value:   "not_wired",
			Source:  operatorSourcePrometheusNeeded,
			Summary: "Lifecycle cleanup counters need a dedicated bounded maintenance read model before UI can mark them healthy.",
		},
		Notes: []string{
			"Upload rejection counters here are persisted-job proxies; pre-job HTTP rejects remain in private metrics.",
			"Frontend and VK surfaces do not run ffmpeg/ffprobe or provider calls.",
		},
	})
}

func (h *Handler) getOperatorConfigHealth(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	writeJSON(w, http.StatusOK, OperatorConfigHealthDTO{
		GeneratedAt:     now,
		Environment:     h.cfg.Runtime.Environment,
		Flags:           h.operatorConfigFlags(),
		ProviderClasses: h.operatorRuntimeProviderDTOs(),
		VideoRoutes:     h.operatorVideoRouteDTOs(),
		Notes: []string{
			"This endpoint exposes non-secret flags only; paths, URLs, tokens and raw model IDs are intentionally omitted.",
		},
	})
}

func (h *Handler) mediaSafetyOverviewCard(ctx context.Context) OverviewCardDTO {
	invalid := h.countJobsByFilter(ctx, domain.JobFilter{ErrorCode: domain.JobErrMediaProviderOutputInvalid})
	overloaded := h.countJobsByFilter(ctx, domain.JobFilter{ErrorCode: domain.JobErrMediaOverloadedRetryLater})
	status := overviewStatusOK
	if invalid.err || overloaded.err || invalid.count > 0 || overloaded.count > 0 {
		status = overviewStatusWarning
	}
	return OverviewCardDTO{
		ID:      "media_safety",
		Title:   "Media safety",
		Status:  status,
		Summary: "Media safety uses config policy plus bounded job-derived risk proxies; private metrics remain in Grafana.",
		Metrics: []OverviewMetricDTO{
			{Label: "probe policy", Value: h.cfg.Runtime.MediaVideoProbePolicy},
			{Label: "transcode policy", Value: h.cfg.Runtime.MediaVideoTranscodePolicy},
			{Label: "invalid outputs", Value: invalid.display(), Status: metricWarningWhenPositive(invalid)},
			{Label: "overloaded media", Value: overloaded.display(), Status: metricWarningWhenPositive(overloaded)},
		},
	}
}

func (h *Handler) operatorProviderHealth(ctx context.Context, class RuntimeProviderClass) OperatorProviderHealthDTO {
	modality := domain.Modality(class.Modality)
	rateLimited := h.countJobsByFilter(ctx, domain.JobFilter{Modality: modality, ErrorCode: string(domain.ProviderErrRateLimited)})
	providerFailed := h.countJobsByFilter(ctx, domain.JobFilter{Status: domain.JobStatusProviderFailed, Modality: modality})
	invalidOutput := h.countJobsByFilter(ctx, domain.JobFilter{Modality: modality, ErrorCode: domain.JobErrMediaProviderOutputInvalid})
	health := overviewStatusOK
	if rateLimited.err || providerFailed.err || invalidOutput.err {
		health = "unknown"
	} else if rateLimited.count > 0 || providerFailed.count > 0 || invalidOutput.count > 0 {
		health = overviewStatusWarning
	}
	fallbackState := "single_provider"
	if len(h.operatorProviderClassNames()) > 1 {
		fallbackState = "configured"
	}
	return OperatorProviderHealthDTO{
		ProviderClass:       class.ProviderClass,
		ModelClass:          class.ModelClass,
		Modality:            class.Modality,
		Health:              health,
		CircuitState:        overviewStatusNotWired,
		RateLimitCount:      rateLimited.count,
		ProviderFailedCount: providerFailed.count,
		InvalidOutputCount:  invalidOutput.count,
		FallbackState:       fallbackState,
		ContractConfigured:  class.ContractConfigured,
		QualityGuardEnabled: h.cfg.Runtime.MediaProviderQualityGuardEnabled,
		Source:              operatorSourceJobModalityProxy,
	}
}

func (h *Handler) operatorProviderFallback() OperatorProviderFallbackDTO {
	providers := h.operatorProviderClassNames()
	if len(providers) == 0 {
		return OperatorProviderFallbackDTO{
			Status:  overviewStatusNotWired,
			Summary: "No provider class is configured in the safe runtime snapshot.",
		}
	}
	if len(providers) == 1 {
		return OperatorProviderFallbackDTO{
			Status:          overviewStatusWarning,
			ProviderClasses: providers,
			Summary:         "Only one provider class is visible; provider fallback is not available from this snapshot.",
		}
	}
	return OperatorProviderFallbackDTO{
		Status:          overviewStatusOK,
		ProviderClasses: providers,
		Summary:         "Multiple provider classes are configured; fallback availability still depends on worker routing and model capability.",
	}
}

func (h *Handler) operatorProviderWasteSignal(ctx context.Context) OperatorRiskSignalDTO {
	count := combineOverviewCounts(
		h.countJobsByFilter(ctx, domain.JobFilter{ErrorCode: domain.JobErrMediaProviderOutputInvalid}),
		h.countJobsByFilter(ctx, domain.JobFilter{ErrorCode: domain.JobErrMediaProcessingUnavailable}),
	)
	return OperatorRiskSignalDTO{
		ID:      "provider_waste",
		Title:   "Provider success without product value",
		Status:  countRiskStatus(count),
		Value:   count.display(),
		Source:  operatorSourceJobSnapshot,
		Summary: "Risk proxy: media jobs that ended in invalid output or processing unavailable before successful delivery/capture.",
	}
}

func (h *Handler) operatorDeliveryCaptureGapSignal(ctx context.Context) OperatorRiskSignalDTO {
	count := h.countDeliveryCaptureGap(ctx)
	return OperatorRiskSignalDTO{
		ID:      "delivery_capture_gap",
		Title:   "Delivery/capture gap",
		Status:  countRiskStatus(count),
		Value:   count.display(),
		Source:  operatorSourceJobSnapshot,
		Summary: "Jobs with reserved credits that are ready/delivering/succeeded but not fully captured.",
	}
}

func (h *Handler) operatorUploadRejectSignal(ctx context.Context, errorCode, title string) OperatorRiskSignalDTO {
	count := h.countJobsByFilter(ctx, domain.JobFilter{Status: domain.JobStatusRejected, ErrorCode: errorCode})
	return OperatorRiskSignalDTO{
		ID:      errorCode,
		Title:   title,
		Status:  countRiskStatus(count),
		Value:   count.display(),
		Source:  operatorSourceJobSnapshot,
		Summary: "Persisted rejected-job count. HTTP-level pre-job rejects are available through private metrics, not this admin API.",
	}
}

func (h *Handler) operatorInvalidOutputSignal(ctx context.Context) OperatorRiskSignalDTO {
	count := h.countJobsByFilter(ctx, domain.JobFilter{ErrorCode: domain.JobErrMediaProviderOutputInvalid})
	return OperatorRiskSignalDTO{
		ID:      "invalid_provider_output",
		Title:   "Invalid provider output",
		Status:  countRiskStatus(count),
		Value:   count.display(),
		Source:  operatorSourceJobSnapshot,
		Summary: "Generated media failed product safety checks before delivery; credits should not be captured for these failures.",
	}
}

func (h *Handler) operatorFastPathSignal() OperatorRiskSignalDTO {
	status := overviewStatusNotWired
	value := "not_wired"
	summary := "Fast path counters live in private Prometheus and are not exposed through this admin read model yet."
	if h.cfg.Runtime.MediaVideoProbePolicy == platformconfig.MediaVideoProbePolicyProbeRequired &&
		h.cfg.Runtime.MediaDeliverRawProviderVideo == platformconfig.MediaDeliverRawProviderVideoIfProbePassed {
		status = overviewStatusOK
		value = "policy_ready"
		summary = "Policy allows probe-passed delivery-ready provider video without default ffmpeg transcode; live counts remain in private metrics."
	}
	return OperatorRiskSignalDTO{
		ID:      "fast_path",
		Title:   "Fast path vs fallback",
		Status:  status,
		Value:   value,
		Source:  operatorSourceRuntimeConfig,
		Summary: summary,
	}
}

func (h *Handler) operatorTranscodeFallbackSignal(ctx context.Context) OperatorRiskSignalDTO {
	count := h.countJobsByFilter(ctx, domain.JobFilter{ErrorCode: domain.JobErrMediaProcessingUnavailable})
	status := countRiskStatus(count)
	value := count.display()
	if h.cfg.Runtime.MediaVideoTranscodePolicy == platformconfig.MediaVideoTranscodePolicyNever && count.count == 0 && !count.err {
		value = "disabled"
	}
	return OperatorRiskSignalDTO{
		ID:      "transcode_fallback",
		Title:   "Transcode fallback",
		Status:  status,
		Value:   value,
		Source:  operatorSourceJobSnapshot,
		Summary: "Processing-unavailable failures are a bounded proxy for failed fallback/transcode paths; live ffmpeg counters stay private.",
	}
}

func (h *Handler) operatorMediaPolicy() OperatorMediaPolicyDTO {
	return OperatorMediaPolicyDTO{
		PipelineEnabled:                 h.cfg.Runtime.MediaPipelineEnabled,
		ProbePolicy:                     h.cfg.Runtime.MediaVideoProbePolicy,
		TranscodePolicy:                 h.cfg.Runtime.MediaVideoTranscodePolicy,
		RawProviderVideoPolicy:          h.cfg.Runtime.MediaDeliverRawProviderVideo,
		ReferenceUploadsEnabled:         h.cfg.Runtime.MediaReferenceUploadsEnabled,
		WebPReferenceEnabled:            h.cfg.Runtime.MediaReferenceWebPEnabled,
		MaxImageUploadBytes:             h.cfg.Runtime.MediaMaxImageUploadBytes,
		MaxImagePixels:                  h.cfg.Runtime.MediaMaxImagePixels,
		MaxVideoSizeBytes:               h.cfg.Runtime.MediaMaxVideoSizeBytes,
		MaxVideoDurationSec:             h.cfg.Runtime.MediaMaxVideoDurationSec,
		MaxConcurrentUploads:            h.cfg.Runtime.MediaMaxConcurrentUploads,
		MaxConcurrentProbes:             h.cfg.Runtime.MediaMaxConcurrentProbes,
		MaxConcurrentTranscodes:         h.cfg.Runtime.MediaMaxConcurrentTranscodes,
		MaxPendingVariants:              h.cfg.Runtime.MediaMaxPendingVariants,
		QueueDegradeThreshold:           h.cfg.Runtime.MediaQueueDegradeThreshold,
		ProviderMaxAttemptsPerJob:       h.cfg.Runtime.MediaProviderMaxAttemptsPerJob,
		ProviderFallbackBudgetPerJob:    h.cfg.Runtime.MediaProviderFallbackBudget,
		ProviderQualityGuardEnabled:     h.cfg.Runtime.MediaProviderQualityGuardEnabled,
		ProviderQualityDegradedFailures: h.cfg.Runtime.MediaProviderQualityDegradedFailures,
		ProviderQualityDisabledFailures: h.cfg.Runtime.MediaProviderQualityDisabledFailures,
	}
}

func (h *Handler) operatorConfigFlags() []OperatorConfigFlagDTO {
	flags := []OperatorConfigFlagDTO{
		{Key: "APP_ENV", Value: h.cfg.Runtime.Environment, Status: overviewStatusOK, Summary: "Runtime environment label."},
		{Key: "PAYMENT_PROVIDER", Value: h.cfg.Runtime.PaymentProvider, Status: overviewStatusOK, Summary: "Payment provider class only; credentials and URLs are omitted."},
		{Key: "PAYMENT_WEBHOOK_REQUIRE_HTTPS", Value: boolString(h.cfg.Runtime.PaymentWebhookHTTPSRequired), Status: boolStatus(h.cfg.Runtime.PaymentWebhookHTTPSRequired), Summary: "Provider webhooks must be HTTPS or trusted proxy forwarded."},
		{Key: "MEDIA_PIPELINE_ENABLED", Value: boolString(h.cfg.Runtime.MediaPipelineEnabled), Status: overviewStatusOK, Summary: "Worker-owned media pipeline switch."},
		{Key: "MEDIA_VIDEO_PROBE_POLICY", Value: h.cfg.Runtime.MediaVideoProbePolicy, Status: probePolicyStatus(h.cfg.Runtime.MediaVideoProbePolicy), Summary: "Generated video validation policy."},
		{Key: "MEDIA_VIDEO_TRANSCODE_POLICY", Value: h.cfg.Runtime.MediaVideoTranscodePolicy, Status: transcodePolicyStatus(h.cfg.Runtime.MediaVideoTranscodePolicy), Summary: "ffmpeg is not expected on the default safe path."},
		{Key: "MEDIA_DELIVER_RAW_PROVIDER_VIDEO", Value: h.cfg.Runtime.MediaDeliverRawProviderVideo, Status: rawVideoPolicyStatus(h.cfg.Runtime.MediaDeliverRawProviderVideo), Summary: "Raw provider delivery is allowed only when policy and probe permit it."},
		{Key: "MEDIA_REFERENCE_UPLOADS_ENABLED", Value: boolString(h.cfg.Runtime.MediaReferenceUploadsEnabled), Status: overviewStatusOK, Summary: "Reference image upload feature flag."},
		{Key: "MEDIA_REFERENCE_WEBP_ENABLED", Value: boolString(h.cfg.Runtime.MediaReferenceWebPEnabled), Status: overviewStatusOK, Summary: "WebP references stay disabled by default unless explicitly enabled."},
		{Key: "MEDIA_PROVIDER_QUALITY_GUARD_ENABLED", Value: boolString(h.cfg.Runtime.MediaProviderQualityGuardEnabled), Status: overviewStatusOK, Summary: "Runtime quality guard configuration; live state remains worker-owned."},
		{Key: "FEATURE_VIDEO_ROUTER_ENABLED", Value: boolString(h.videoRouterEnabled()), Status: boolStatus(h.videoRouterEnabled()), Summary: "Public video route aliases are accepted only when this router flag is enabled."},
	}
	return flags
}

func (h *Handler) operatorRuntimeProviderDTOs() []OperatorRuntimeProviderDTO {
	out := make([]OperatorRuntimeProviderDTO, 0, len(h.cfg.Runtime.ProviderClasses))
	for _, class := range h.cfg.Runtime.ProviderClasses {
		out = append(out, OperatorRuntimeProviderDTO{
			ProviderClass:      class.ProviderClass,
			ModelClass:         class.ModelClass,
			Modality:           class.Modality,
			ContractConfigured: class.ContractConfigured,
		})
	}
	return out
}

func (h *Handler) operatorVideoRouteDTOs() []OperatorVideoRouteDTO {
	out := make([]OperatorVideoRouteDTO, 0, len(h.cfg.Runtime.VideoRoutes))
	for _, route := range h.cfg.Runtime.VideoRoutes {
		out = append(out, OperatorVideoRouteDTO{
			Alias:                  route.Alias,
			ProviderClass:          route.ProviderClass,
			ModelClass:             route.ModelClass,
			Status:                 route.Status,
			Reason:                 route.Reason,
			Enabled:                route.Enabled,
			ProviderEnabled:        route.ProviderEnabled,
			ProviderConfigured:     route.ProviderConfigured,
			ProviderBaseConfigured: route.ProviderBaseConfigured,
			CostConfigured:         route.CostConfigured,
			RequiresStartImage:     route.RequiresStartImage,
			SupportsReferenceImage: route.SupportsReferenceImage,
			MaxReferenceImages:     route.MaxReferenceImages,
			AllowedDurationsSec:    append([]int(nil), route.AllowedDurationsSec...),
			AllowedResolutions:     append([]string(nil), route.AllowedResolutions...),
		})
	}
	return out
}

func (h *Handler) videoRouterEnabled() bool {
	for _, route := range h.cfg.Runtime.VideoRoutes {
		if route.Reason != "router_disabled" {
			return true
		}
	}
	return false
}

func (h *Handler) operatorProviderClassNames() []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(h.cfg.Runtime.ProviderClasses))
	for _, class := range h.cfg.Runtime.ProviderClasses {
		if class.ProviderClass == "" {
			continue
		}
		if _, exists := seen[class.ProviderClass]; exists {
			continue
		}
		seen[class.ProviderClass] = struct{}{}
		out = append(out, class.ProviderClass)
	}
	sort.Strings(out)
	return out
}

func (h *Handler) countJobsByFilter(ctx context.Context, filter domain.JobFilter) overviewCount {
	if h.deps.Jobs == nil {
		return overviewCount{err: true}
	}
	jobs, err := h.deps.Jobs.List(ctx, filter, overviewCountLimit, 0)
	if err != nil {
		return overviewCount{err: true}
	}
	return overviewCount{count: boundedOverviewCount(len(jobs))}
}

func (h *Handler) countDeliveryCaptureGap(ctx context.Context) overviewCount {
	if h.deps.Jobs == nil {
		return overviewCount{err: true}
	}
	var total int
	for _, status := range []domain.JobStatus{
		domain.JobStatusProviderSucceeded,
		domain.JobStatusResultReady,
		domain.JobStatusDelivering,
		domain.JobStatusSucceeded,
	} {
		jobs, err := h.deps.Jobs.List(ctx, domain.JobFilter{Status: status}, overviewCountLimit, 0)
		if err != nil {
			return overviewCount{err: true}
		}
		for _, job := range jobs {
			if job.Modality != domain.ModalityImage && job.Modality != domain.ModalityVideo {
				continue
			}
			if job.CostReserved > 0 && job.CostCaptured < job.CostReserved {
				total++
			}
			if total >= maxLimit {
				return overviewCount{count: maxLimit}
			}
		}
	}
	return overviewCount{count: total}
}

func runtimeProviderClasses(cfg platformconfig.Config) []RuntimeProviderClass {
	classes := map[string]RuntimeProviderClass{}
	add := func(provider, modality string, contract bool, modelClasses ...string) {
		providerClass := safeProviderClass(provider)
		if providerClass == "" || providerClass == "unknown_provider" || modality == "" {
			return
		}
		modelClass := ""
		for _, candidate := range modelClasses {
			if modelClass = safeRuntimeToken(candidate, ""); modelClass != "" {
				break
			}
		}
		if modelClass == "" {
			modelClass = curatedModelClass(providerClass, modality)
		}
		key := providerClass + "|" + modelClass + "|" + modality
		current, exists := classes[key]
		if exists && current.ContractConfigured {
			return
		}
		classes[key] = RuntimeProviderClass{
			ProviderClass:      providerClass,
			ModelClass:         modelClass,
			Modality:           modality,
			ContractConfigured: contract,
		}
	}

	defaultProviders := compactStrings(append([]string{cfg.Provider}, cfg.ProviderChain...))
	if len(defaultProviders) == 0 {
		defaultProviders = []string{"mock"}
	}
	for _, provider := range defaultProviders {
		add(provider, string(domain.ModalityText), false)
		add(provider, string(domain.ModalityImage), false)
		if strings.TrimSpace(cfg.VideoProvider) == "" && strings.TrimSpace(cfg.VideoModel) != "" {
			add(provider, string(domain.ModalityVideo), false)
		}
	}
	add(cfg.ImageProvider, string(domain.ModalityImage), false)
	add(cfg.VideoProvider, string(domain.ModalityVideo), false)
	if strings.TrimSpace(cfg.DeepInfraVideoModel) != "" {
		add(string(domain.ProviderDeepInfra), string(domain.ModalityVideo), false)
	}
	if strings.TrimSpace(cfg.OpenAIVideoModel) != "" {
		add(string(domain.ProviderOpenAI), string(domain.ModalityVideo), false)
	}
	for _, contract := range cfg.MediaProviderContracts {
		modality := string(contract.Modality)
		modelClass := safeRuntimeToken(contract.ModelClass, "")
		add(string(contract.Provider), modality, true, modelClass)
	}

	out := make([]RuntimeProviderClass, 0, len(classes))
	for _, class := range classes {
		out = append(out, class)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProviderClass != out[j].ProviderClass {
			return out[i].ProviderClass < out[j].ProviderClass
		}
		if out[i].Modality != out[j].Modality {
			return out[i].Modality < out[j].Modality
		}
		return out[i].ModelClass < out[j].ModelClass
	})
	return out
}

func runtimeVideoRoutes(cfg platformconfig.Config) []RuntimeVideoRoute {
	enabledRoutes := map[domain.VideoRouteAlias]bool{
		domain.VideoRouteHailuo23Fast:     cfg.FeatureVideoRouteHailuo23FastEnabled,
		domain.VideoRouteHailuo23Standard: cfg.FeatureVideoRouteHailuo23StandardEnabled,
		domain.VideoRouteKlingO3Standard:  cfg.FeatureVideoRouteKlingO3StandardEnabled,
		domain.VideoRouteRunwayGen4Turbo:  cfg.FeatureVideoRouteRunwayGen4TurboEnabled,
		domain.VideoRouteSeedance20Fast:   cfg.FeatureVideoRouteSeedance20FastEnabled,
		domain.VideoRouteRunwayGen45:      cfg.FeatureVideoRouteRunwayGen45Enabled,
		domain.VideoRouteMockTextToVideo:  cfg.FeatureVideoRouteMockTextToVideoEnabled,
	}
	out := make([]RuntimeVideoRoute, 0, len(enabledRoutes))
	for _, spec := range videorouter.DefaultRouteSpecs() {
		provider := runtimeVideoProviderReadiness(cfg, spec.Provider)
		costConfigured := spec.ProviderCostCreditsFixed > 0 || spec.ProviderCostCreditsPerSecond > 0
		status, reason := runtimeVideoRouteStatus(
			cfg.FeatureVideoRouterEnabled,
			enabledRoutes[spec.Alias],
			provider.enabled,
			provider.configured,
			provider.baseConfigured,
			costConfigured,
		)
		out = append(out, RuntimeVideoRoute{
			Alias:                  safeRuntimeToken(string(spec.Alias), "unknown_route"),
			ProviderClass:          safeProviderClass(string(spec.Provider)),
			ModelClass:             safeRuntimeToken(spec.ModelClass, "unknown_model_class"),
			Status:                 status,
			Reason:                 reason,
			Enabled:                enabledRoutes[spec.Alias],
			ProviderEnabled:        provider.enabled,
			ProviderConfigured:     provider.configured,
			ProviderBaseConfigured: provider.baseConfigured,
			CostConfigured:         costConfigured,
			RequiresStartImage:     spec.RequiresStartImage,
			SupportsReferenceImage: spec.SupportsReferenceImage,
			MaxReferenceImages:     spec.MaxReferenceImages,
			AllowedDurationsSec:    append([]int(nil), spec.AllowedDurationsSec...),
			AllowedResolutions:     compactStrings(spec.AllowedResolutions),
		})
	}
	return out
}

type runtimeVideoProviderState struct {
	enabled        bool
	configured     bool
	baseConfigured bool
}

func runtimeVideoProviderReadiness(cfg platformconfig.Config, provider domain.ProviderName) runtimeVideoProviderState {
	switch provider {
	case domain.ProviderAPIMart:
		return runtimeVideoProviderState{
			enabled:        cfg.APIMartProviderEnabled,
			configured:     strings.TrimSpace(cfg.APIMartAPIKey) != "",
			baseConfigured: strings.TrimSpace(cfg.APIMartBaseURL) != "",
		}
	case domain.ProviderPoYo:
		return runtimeVideoProviderState{
			enabled:        cfg.PoYoProviderEnabled,
			configured:     strings.TrimSpace(cfg.PoYoAPIKey) != "",
			baseConfigured: strings.TrimSpace(cfg.PoYoBaseURL) != "",
		}
	case domain.ProviderRunway:
		return runtimeVideoProviderState{
			enabled:        cfg.RunwayProviderEnabled,
			configured:     strings.TrimSpace(cfg.RunwayMLAPISecret) != "",
			baseConfigured: strings.TrimSpace(cfg.RunwayMLBaseURL) != "",
		}
	case domain.ProviderMock:
		videoProvider := strings.TrimSpace(cfg.VideoProvider)
		ready := cfg.IsLoadTest() &&
			strings.EqualFold(strings.TrimSpace(cfg.Provider), string(domain.ProviderMock)) &&
			(videoProvider == "" || strings.EqualFold(videoProvider, string(domain.ProviderMock)))
		for _, provider := range cfg.ProviderChain {
			if strings.TrimSpace(provider) != "" && !strings.EqualFold(provider, string(domain.ProviderMock)) {
				ready = false
				break
			}
		}
		return runtimeVideoProviderState{
			enabled:        ready,
			configured:     ready,
			baseConfigured: ready,
		}
	default:
		return runtimeVideoProviderState{}
	}
}

func runtimeVideoRouteStatus(routerEnabled, routeEnabled, providerEnabled, providerConfigured, providerBaseConfigured, costConfigured bool) (string, string) {
	switch {
	case !routerEnabled:
		return overviewStatusNotWired, "router_disabled"
	case !routeEnabled:
		return overviewStatusNotWired, "route_flag_off"
	case !providerEnabled:
		return overviewStatusWarning, "provider_disabled"
	case !providerConfigured || !providerBaseConfigured:
		return overviewStatusWarning, "provider_unconfigured"
	case !costConfigured:
		return overviewStatusWarning, "cost_unavailable"
	default:
		return overviewStatusOK, "ready"
	}
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func combineOverviewCounts(counts ...overviewCount) overviewCount {
	var total int
	for _, count := range counts {
		if count.err {
			return overviewCount{err: true}
		}
		total += count.count
		if total >= maxLimit {
			return overviewCount{count: maxLimit}
		}
	}
	return overviewCount{count: total}
}

func countRiskStatus(count overviewCount) string {
	if count.err {
		return "unknown"
	}
	if count.count > 0 {
		return overviewStatusWarning
	}
	return overviewStatusOK
}

func safeRuntimeToken(value, fallback string) string {
	value = sanitizeOperatorToken(strings.ToLower(strings.TrimSpace(value)))
	if value == "" {
		return fallback
	}
	return value
}

func safeProviderClass(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "mock":
		return "mock"
	case "openai":
		return "openai"
	case "deepinfra":
		return "deepinfra"
	case "apimart":
		return "apimart"
	case "poyo":
		return "poyo"
	case "runway":
		return "runway"
	case "yookassa":
		return "yookassa"
	case "":
		return "unknown_provider"
	default:
		return "custom_provider"
	}
}

func curatedModelClass(providerClass, modality string) string {
	switch modality {
	case string(domain.ModalityText), string(domain.ModalityImage), string(domain.ModalityVideo):
	default:
		modality = "unknown"
	}
	switch providerClass {
	case "mock", "openai", "deepinfra":
		return providerClass + "_" + modality
	default:
		return "custom_" + modality
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func boolStatus(value bool) string {
	if value {
		return overviewStatusOK
	}
	return overviewStatusWarning
}

func probePolicyStatus(policy string) string {
	switch policy {
	case platformconfig.MediaVideoProbePolicyProbeRequired:
		return overviewStatusOK
	case platformconfig.MediaVideoProbePolicyTrustedProvider:
		return overviewStatusWarning
	default:
		return overviewStatusWarning
	}
}

func transcodePolicyStatus(policy string) string {
	if policy == platformconfig.MediaVideoTranscodePolicyAlways {
		return overviewStatusWarning
	}
	return overviewStatusOK
}

func rawVideoPolicyStatus(policy string) string {
	switch policy {
	case platformconfig.MediaDeliverRawProviderVideoIfProbePassed, platformconfig.MediaDeliverRawProviderVideoNever:
		return overviewStatusOK
	default:
		return overviewStatusWarning
	}
}
