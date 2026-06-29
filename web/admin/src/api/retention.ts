import type { AdminClient } from "./adminClient";
import { createIdempotencyKey } from "./adminClient";

export type OperatorRetentionTableDTO = {
  table_name: string;
  retention_class: string;
  total_rows: number;
  expired_rows: number;
  redacted_rows: number;
  deleted_rows: number;
  oldest_hot_at?: string;
  oldest_hot_age_seconds: number;
  oldest_expired_at?: string;
};

export type OperatorOldestHotRowDTO = {
  table_name: string;
  retention_class: string;
  count: number;
  oldest_at?: string;
  age_seconds: number;
};

export type OperatorOrphanArtifactDTO = {
  artifact_tier: string;
  lifecycle_class: string;
  status: string;
  media_type: string;
  count: number;
  bytes: number;
  oldest_at?: string;
  oldest_age_seconds: number;
};

export type OperatorOrphanArtifactsDTO = {
  generated_at: string;
  total: number;
  bytes: number;
  items: OperatorOrphanArtifactDTO[];
  notes?: string[];
};

export type OperatorRetentionStatusDTO = {
  generated_at: string;
  retention: OperatorRetentionTableDTO[];
  oldest_hot_rows: OperatorOldestHotRowDTO[];
  orphan_artifacts: OperatorOrphanArtifactsDTO;
  notes?: string[];
};

export type OperatorRetentionDryRunItemDTO = {
  action: string;
  table_name: string;
  retention_class: string;
  count: number;
  bytes: number;
  oldest_at?: string;
  oldest_age_seconds: number;
};

export type OperatorRetentionDryRunDTO = {
  generated_at: string;
  items: OperatorRetentionDryRunItemDTO[];
  notes?: string[];
};

export type OperatorRetentionCleanupDTO = OperatorRetentionStatusDTO & {
  completed: boolean;
};

export type OperatorAnalyticsStatusItemDTO = {
  table_name: string;
  status: string;
  rows: number;
  latest_activity_date?: string;
  last_updated_at?: string;
  last_updated_age_seconds: number;
};

export type OperatorAnalyticsStatusDTO = {
  generated_at: string;
  items: OperatorAnalyticsStatusItemDTO[];
  notes?: string[];
};

export async function fetchRetentionStatus(client: AdminClient, signal?: AbortSignal): Promise<OperatorRetentionStatusDTO> {
  return client.request<OperatorRetentionStatusDTO>("/admin/retention/operator/status", { signal });
}

export async function fetchRetentionDryRun(
  client: AdminClient,
  limit = 50,
  signal?: AbortSignal,
): Promise<OperatorRetentionDryRunDTO> {
  return client.request<OperatorRetentionDryRunDTO>(`/admin/retention/operator/dry-run?limit=${encodeURIComponent(String(limit))}`, {
    signal,
  });
}

export async function runRetentionCleanup(client: AdminClient): Promise<OperatorRetentionCleanupDTO> {
  return client.request<OperatorRetentionCleanupDTO>("/admin/retention/operator/run-cleanup", {
    method: "POST",
    body: {},
    idempotencyKey: createIdempotencyKey("retention_cleanup"),
  });
}

export async function fetchAnalyticsStatus(client: AdminClient, signal?: AbortSignal): Promise<OperatorAnalyticsStatusDTO> {
  return client.request<OperatorAnalyticsStatusDTO>("/admin/analytics/operator/status", { signal });
}
