export type PaginationDTO = {
  limit: number;
  offset: number;
  count: number;
  has_more: boolean;
  cursor?: string;
  next_cursor?: string;
};

export type OperatorQueueMetricDTO = {
  label: string;
  value: string;
  status: "ok" | "warning" | "critical" | "unknown";
};

export type OperatorQueueNotWiredDTO = {
  status: "not_wired";
  reason: string;
};

export type OperatorDLQSummaryDTO = {
  status: "ok" | "warning" | "critical" | "unknown";
  reason: string;
  retryable_count: number;
  terminal_count: number;
  batch_replay_limit: number;
};

export type OperatorQueueSummaryDTO = {
  generated_at: string;
  degradation_state: "normal" | "watch" | "degraded" | "unknown";
  backlog: OperatorQueueMetricDTO[];
  oldest_queued_age_seconds?: number;
  retry_count: number;
  dlq: OperatorDLQSummaryDTO;
  provider_circuit: OperatorQueueNotWiredDTO;
  notes?: string[];
};

export type OperatorJobListItemDTO = {
  lookup_id: string;
  display_id: string;
  correlation_ref?: string;
  operation: string;
  modality: string;
  status: string;
  error_class?: string;
  cost_estimate: number;
  cost_reserved: number;
  cost_captured: number;
  input_count: number;
  output_count: number;
  created_at: string;
  updated_at: string;
  age_seconds: number;
};

export type OperatorJobsDTO = {
  generated_at: string;
  items: OperatorJobListItemDTO[];
  pagination: PaginationDTO;
  queue: OperatorQueueSummaryDTO;
};

export type OperatorReservationDTO = {
  status: string;
  amount: number;
  expires_at: string;
  updated_at: string;
};

export type OperatorDeliverySummaryDTO = {
  status: string;
  attempts: number;
  retry_count: number;
  last_error_class?: string;
  last_artifact_ref?: string;
  last_delivery_type?: string;
  last_delivery_status?: string;
};

export type OperatorDeliveryAttemptDTO = {
  type: string;
  status: string;
  attempt_no: number;
  error_class?: string;
  artifact_ref?: string;
  created_at: string;
  updated_at: string;
};

export type OperatorJobDetailDTO = {
  job: OperatorJobListItemDTO;
  allowed_next_statuses: string[];
  artifacts: {
    input_refs: string[];
    output_refs: string[];
  };
  reservation?: OperatorReservationDTO;
  delivery: OperatorDeliverySummaryDTO;
  delivery_events: OperatorDeliveryAttemptDTO[];
};

export type OperatorDLQItemDTO = {
  job: OperatorJobListItemDTO;
  attempt_count: number;
  provider_task_count: number;
  last_error_class?: string;
  last_provider_class?: string;
  safe_replay: boolean;
  replay_blocked_reason?: string;
  replay_target: string;
};

export type OperatorDLQReplayPolicyDTO = {
  single_allowed_statuses: string[];
  batch_limit: number;
  batch_skips_paid_provider: boolean;
  notes?: string[];
};

export type OperatorDLQDTO = {
  generated_at: string;
  items: OperatorDLQItemDTO[];
  pagination: PaginationDTO;
  replay: OperatorDLQReplayPolicyDTO;
  notes?: string[];
};

export type OperatorDLQReplayRequestDTO = {
  job_ids?: string[];
  limit?: number;
  error_class?: string;
  allow_paid_provider?: boolean;
};

export type OperatorDLQReplayItemDTO = {
  lookup_id: string;
  display_id: string;
  status: string;
  result: string;
  reason?: string;
};

export type OperatorDLQReplayResultDTO = {
  generated_at: string;
  requested: number;
  replayed: OperatorDLQReplayItemDTO[];
  skipped: OperatorDLQReplayItemDTO[];
  batch_limit: number;
};
