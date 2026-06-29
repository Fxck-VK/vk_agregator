import type { OperatorQueueSummaryDTO } from "./jobs";

export type OperatorRiskStatus = "ok" | "warning" | "critical" | "unknown" | "not_wired";

export type OperatorProviderHealthDTO = {
  provider_class: string;
  service_type: string;
  model_class: string;
  modality: string;
  health: OperatorRiskStatus;
  circuit_state: OperatorRiskStatus;
  quota_state: string;
  cooldown_state: string;
  rate_limit_count: number;
  provider_failed_count: number;
  invalid_output_count: number;
  observed_total_count: number;
  error_rate_percent: number;
  latency_p95_ms: number;
  in_flight_count: number;
  latest_error_class?: string;
  latest_error_at?: string;
  fallback_state: string;
  contract_configured: boolean;
  quality_guard_enabled: boolean;
  source: string;
};

export type OperatorVideoRouteDTO = {
  alias: string;
  provider_class: string;
  model_class: string;
  status: OperatorRiskStatus;
  reason: string;
  enabled: boolean;
  provider_enabled: boolean;
  provider_configured: boolean;
  provider_base_configured: boolean;
  cost_configured: boolean;
  requires_start_image: boolean;
  supports_reference_image: boolean;
  max_reference_images?: number;
  allowed_durations_sec?: number[];
  allowed_resolutions?: string[];
};

export type OperatorProviderFallbackDTO = {
  status: OperatorRiskStatus;
  provider_classes?: string[];
  summary: string;
};

export type OperatorRiskSignalDTO = {
  id: string;
  title: string;
  status: OperatorRiskStatus;
  value: string;
  source: string;
  summary: string;
};

export type OperatorNotWiredSignalDTO = {
  status: "not_wired";
  source: string;
  summary: string;
};

export type OperatorProviderControlRoomDTO = {
  generated_at: string;
  providers: OperatorProviderHealthDTO[];
  video_routes: OperatorVideoRouteDTO[];
  fallback: OperatorProviderFallbackDTO;
  provider_waste: OperatorRiskSignalDTO;
  delivery_capture_gap: OperatorRiskSignalDTO;
  circuit: OperatorNotWiredSignalDTO;
  notes?: string[];
};

export type OperatorMediaPolicyDTO = {
  pipeline_enabled: boolean;
  probe_policy: string;
  transcode_policy: string;
  raw_provider_video_policy: string;
  reference_uploads_enabled: boolean;
  webp_reference_enabled: boolean;
  max_image_upload_bytes: number;
  max_image_pixels: number;
  max_video_size_bytes: number;
  max_video_duration_sec: number;
  max_concurrent_uploads: number;
  max_concurrent_probes: number;
  max_concurrent_transcodes: number;
  max_pending_variants: number;
  queue_degrade_threshold: number;
  provider_max_attempts_per_job: number;
  provider_fallback_budget_per_job: number;
  provider_quality_guard_enabled: boolean;
  provider_quality_degraded_failures: number;
  provider_quality_disabled_failures: number;
};

export type OperatorMediaSafetyDTO = {
  generated_at: string;
  policy: OperatorMediaPolicyDTO;
  uploads: OperatorRiskSignalDTO[];
  queue: OperatorQueueSummaryDTO;
  processing: OperatorRiskSignalDTO[];
  cleanup: OperatorRiskSignalDTO;
  notes?: string[];
};

export type OperatorConfigFlagDTO = {
  key: string;
  value: string;
  status: OperatorRiskStatus;
  summary: string;
};

export type OperatorRuntimeProviderDTO = {
  provider_class: string;
  model_class: string;
  modality: string;
  contract_configured: boolean;
};

export type OperatorConfigHealthDTO = {
  generated_at: string;
  environment: string;
  flags: OperatorConfigFlagDTO[];
  provider_classes: OperatorRuntimeProviderDTO[];
  video_routes: OperatorVideoRouteDTO[];
  notes?: string[];
};
