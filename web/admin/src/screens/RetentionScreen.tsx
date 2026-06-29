import { useEffect, useMemo, useState } from "react";
import type { AdminApiError, AdminClient } from "../api/adminClient";
import { toSafeAdminError } from "../api/adminClient";
import {
  fetchAnalyticsStatus,
  fetchRetentionDryRun,
  fetchRetentionStatus,
  runRetentionCleanup,
  type OperatorAnalyticsStatusDTO,
  type OperatorAnalyticsStatusItemDTO,
  type OperatorOldestHotRowDTO,
  type OperatorOrphanArtifactDTO,
  type OperatorRetentionDryRunDTO,
  type OperatorRetentionStatusDTO,
  type OperatorRetentionTableDTO,
} from "../api/retention";
import { OperatorConfirmDialog } from "../components/OperatorConfirmDialog";

type RetentionScreenProps = {
  adminTokenSet: boolean;
  client: AdminClient;
};

type RetentionState = {
  analytics?: OperatorAnalyticsStatusDTO;
  cleanupMessage?: string;
  dryRun?: OperatorRetentionDryRunDTO;
  error?: AdminApiError;
  loading: boolean;
  mutating: boolean;
  status?: OperatorRetentionStatusDTO;
};

export function RetentionScreen({ adminTokenSet, client }: RetentionScreenProps) {
  const [state, setState] = useState<RetentionState>({ loading: false, mutating: false });
  const [cleanupConfirmOpen, setCleanupConfirmOpen] = useState(false);
  const [reloadNonce, setReloadNonce] = useState(0);

  useEffect(() => {
    if (!adminTokenSet) {
      setState({ loading: false, mutating: false });
      return;
    }
    const controller = new AbortController();
    setState((current) => ({ ...current, error: undefined, loading: true }));
    Promise.all([
      fetchRetentionStatus(client, controller.signal),
      fetchRetentionDryRun(client, 50, controller.signal),
      fetchAnalyticsStatus(client, controller.signal),
    ])
      .then(([status, dryRun, analytics]) =>
        setState((current) => ({
          analytics,
          cleanupMessage: current.cleanupMessage,
          dryRun,
          loading: false,
          mutating: false,
          status,
        })),
      )
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          setState((current) => ({ ...current, error: toSafeAdminError(error), loading: false, mutating: false }));
        }
      });
    return () => controller.abort();
  }, [adminTokenSet, client, reloadNonce]);

  const summary = useMemo(() => summarizeRetention(state.status), [state.status]);

  async function refresh() {
    setReloadNonce((value) => value + 1);
  }

  async function dryRun() {
    setState((current) => ({ ...current, cleanupMessage: undefined, error: undefined, loading: true }));
    try {
      const nextDryRun = await fetchRetentionDryRun(client, 50);
      const nextStatus = await fetchRetentionStatus(client);
      const nextAnalytics = await fetchAnalyticsStatus(client);
      setState((current) => ({
        ...current,
        analytics: nextAnalytics,
        dryRun: nextDryRun,
        loading: false,
        status: nextStatus,
      }));
    } catch (error: unknown) {
      setState((current) => ({ ...current, error: toSafeAdminError(error), loading: false }));
    }
  }

  async function cleanup() {
    setState((current) => ({ ...current, cleanupMessage: undefined, error: undefined, mutating: true }));
    try {
      const cleanupResult = await runRetentionCleanup(client);
      const nextDryRun = await fetchRetentionDryRun(client, 50);
      const nextAnalytics = await fetchAnalyticsStatus(client);
      setState((current) => ({
        ...current,
        analytics: nextAnalytics,
        cleanupMessage: cleanupResult.completed ? "Cleanup completed. Snapshot refreshed." : "Cleanup finished without a completion flag.",
        dryRun: nextDryRun,
        mutating: false,
        status: cleanupResult,
      }));
    } catch (error: unknown) {
      setState((current) => ({ ...current, error: toSafeAdminError(error), mutating: false }));
    }
  }

  if (!adminTokenSet) {
    return (
      <article className="surface panel panel--wide" role="status">
        <p className="eyebrow">Access required</p>
        <h3>Retention console is closed</h3>
        <p>Enter an admin token to inspect safe cleanup counters.</p>
      </article>
    );
  }

  if (state.error) {
    return <SafeErrorPanel error={state.error} />;
  }

  return (
    <div className="ops-stack">
      <section className="surface queue-panel" aria-label="Retention cleanup summary">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Retention</p>
            <h3>Cleanup control</h3>
          </div>
          <span>{state.status ? `Generated ${formatDateTime(state.status.generated_at)}` : "No snapshot yet"}</span>
        </div>
        <div className="retention-actions">
          <button disabled={state.loading || state.mutating} onClick={refresh} type="button">
            Refresh
          </button>
          <button disabled={state.loading || state.mutating} onClick={dryRun} type="button">
            Dry-run cleanup
          </button>
          <button className="button-danger" disabled={state.loading || state.mutating} onClick={() => setCleanupConfirmOpen(true)} type="button">
            {state.mutating ? "Running cleanup" : "Run cleanup"}
          </button>
        </div>
        {state.cleanupMessage ? <p className="retention-message">{state.cleanupMessage}</p> : null}
        <RetentionSummaryGrid summary={summary} />
        <p className="muted">
          Financial tables are not automatic cleanup targets. This screen exposes counters, ages and lifecycle classes only.
        </p>
      </section>

      <section className="surface jobs-list" aria-label="Retention table status">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Hot data</p>
            <h3>{state.status ? `${state.status.retention.length} retention classes` : "Loading retention classes"}</h3>
          </div>
          <span>{state.loading ? "Refreshing" : "safe counters"}</span>
        </div>
        <RetentionTable items={state.status?.retention ?? []} />
      </section>

      <section className="surface jobs-list" aria-label="Dry-run cleanup actions">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Dry-run</p>
            <h3>{state.dryRun ? `${state.dryRun.items.length} cleanup actions` : "Loading cleanup preview"}</h3>
          </div>
          <span>{state.dryRun ? formatDateTime(state.dryRun.generated_at) : "pending"}</span>
        </div>
        <DryRunList dryRun={state.dryRun} />
      </section>

      <section className="surface jobs-list" aria-label="Oldest hot rows">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Aging rows</p>
            <h3>{state.status ? `${state.status.oldest_hot_rows.length} hot row groups` : "Loading hot rows"}</h3>
          </div>
          <span>no raw row data</span>
        </div>
        <OldestRowsList items={state.status?.oldest_hot_rows ?? []} />
      </section>

      <section className="surface jobs-list" aria-label="Orphan artifacts">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Artifacts</p>
            <h3>{`${state.status?.orphan_artifacts.total ?? 0} orphan artifacts`}</h3>
          </div>
          <span>{formatBytes(state.status?.orphan_artifacts.bytes ?? 0)}</span>
        </div>
        <OrphanArtifactsList items={state.status?.orphan_artifacts.items ?? []} />
      </section>

      <section className="surface jobs-list" aria-label="Read model freshness">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Read models</p>
            <h3>{state.analytics ? `${state.analytics.items.length} aggregate tables` : "Loading aggregate status"}</h3>
          </div>
          <span>{state.analytics ? formatDateTime(state.analytics.generated_at) : "pending"}</span>
        </div>
        <AnalyticsStatusList items={state.analytics?.items ?? []} />
      </section>

      <NotesPanel
        notes={[
          ...(state.status?.notes ?? []),
          ...(state.dryRun?.notes ?? []),
          ...(state.analytics?.notes ?? []),
          ...(state.status?.orphan_artifacts.notes ?? []),
        ]}
      />
      {cleanupConfirmOpen ? (
        <OperatorConfirmDialog
          busy={state.mutating}
          confirmLabel="Run cleanup"
          danger
          description="This runs backend retention cleanup. Financial tables stay excluded, but eligible operational rows can be redacted, expired or marked for lifecycle cleanup."
          onCancel={() => setCleanupConfirmOpen(false)}
          onConfirm={() => {
            setCleanupConfirmOpen(false);
            void cleanup();
          }}
          title="Run retention cleanup"
        >
          <dl className="detail-list">
            <div>
              <dt>Preview</dt>
              <dd>{state.dryRun ? `${state.dryRun.items.length} dry-run actions visible` : "Run dry-run first when possible"}</dd>
            </div>
            <div>
              <dt>Protected</dt>
              <dd>ledger, payments, balance history</dd>
            </div>
          </dl>
        </OperatorConfirmDialog>
      ) : null}
    </div>
  );
}

function RetentionSummaryGrid({ summary }: { summary: ReturnType<typeof summarizeRetention> }) {
  return (
    <div className="retention-summary-grid">
      <div className="queue-metric">
        <span>expired rows</span>
        <strong>{formatNumber(summary.expiredRows)}</strong>
      </div>
      <div className="queue-metric">
        <span>old messages</span>
        <strong>{formatNumber(summary.conversationExpiredRows)}</strong>
      </div>
      <div className="queue-metric">
        <span>job events</span>
        <strong>{formatNumber(summary.jobEventExpiredRows)}</strong>
      </div>
      <div className="queue-metric">
        <span>orphan artifacts</span>
        <strong>{formatNumber(summary.orphanArtifacts)}</strong>
      </div>
    </div>
  );
}

function RetentionTable({ items }: { items: OperatorRetentionTableDTO[] }) {
  if (items.length === 0) {
    return <p className="muted">No retention counters are available.</p>;
  }
  return (
    <div className="retention-table" role="table">
      <div className="retention-row retention-row--head" role="row">
        <span>Table</span>
        <span>Class</span>
        <span>Total</span>
        <span>Expired</span>
        <span>Redacted</span>
        <span>Oldest hot</span>
      </div>
      {items.map((item) => (
        <div className="retention-row" key={`${item.table_name}:${item.retention_class}`} role="row">
          <span>{item.table_name}</span>
          <span>{item.retention_class}</span>
          <span>{formatNumber(item.total_rows)}</span>
          <span>{formatNumber(item.expired_rows)}</span>
          <span>{formatNumber(item.redacted_rows)}</span>
          <span>{formatAge(item.oldest_hot_age_seconds, item.oldest_hot_at)}</span>
        </div>
      ))}
    </div>
  );
}

function DryRunList({ dryRun }: { dryRun?: OperatorRetentionDryRunDTO }) {
  if (!dryRun) {
    return <p className="muted">Dry-run data has not loaded yet.</p>;
  }
  if (dryRun.items.length === 0) {
    return <p className="muted">Dry-run found nothing eligible for cleanup.</p>;
  }
  return (
    <div className="event-list">
      {dryRun.items.map((item) => (
        <div key={`${item.action}:${item.table_name}:${item.retention_class}`}>
          <strong>{`${item.action} ${item.table_name}`}</strong>
          <span>{`${item.retention_class} / ${formatNumber(item.count)} rows / ${formatBytes(item.bytes)}`}</span>
          <span>{formatAge(item.oldest_age_seconds, item.oldest_at)}</span>
        </div>
      ))}
    </div>
  );
}

function OldestRowsList({ items }: { items: OperatorOldestHotRowDTO[] }) {
  if (items.length === 0) {
    return <p className="muted">No oldest-row report is available.</p>;
  }
  return (
    <div className="event-list">
      {items.map((item) => (
        <div key={`${item.table_name}:${item.retention_class}`}>
          <strong>{item.table_name}</strong>
          <span>{`${item.retention_class} / ${formatNumber(item.count)} rows`}</span>
          <span>{formatAge(item.age_seconds, item.oldest_at)}</span>
        </div>
      ))}
    </div>
  );
}

function OrphanArtifactsList({ items }: { items: OperatorOrphanArtifactDTO[] }) {
  if (items.length === 0) {
    return <p className="muted">No orphan artifacts are currently reported.</p>;
  }
  return (
    <div className="retention-table" role="table">
      <div className="retention-row retention-row--artifacts retention-row--head" role="row">
        <span>Tier</span>
        <span>Lifecycle</span>
        <span>Status</span>
        <span>Media</span>
        <span>Count</span>
        <span>Bytes</span>
        <span>Oldest</span>
      </div>
      {items.map((item) => (
        <div
          className="retention-row retention-row--artifacts"
          key={`${item.artifact_tier}:${item.lifecycle_class}:${item.status}:${item.media_type}`}
          role="row"
        >
          <span>{item.artifact_tier}</span>
          <span>{item.lifecycle_class}</span>
          <span>{item.status}</span>
          <span>{item.media_type}</span>
          <span>{formatNumber(item.count)}</span>
          <span>{formatBytes(item.bytes)}</span>
          <span>{formatAge(item.oldest_age_seconds, item.oldest_at)}</span>
        </div>
      ))}
    </div>
  );
}

function AnalyticsStatusList({ items }: { items: OperatorAnalyticsStatusItemDTO[] }) {
  if (items.length === 0) {
    return <p className="muted">No read model status is available.</p>;
  }
  return (
    <div className="retention-table" role="table">
      <div className="retention-row retention-row--analytics retention-row--head" role="row">
        <span>Model</span>
        <span>Status</span>
        <span>Rows</span>
        <span>Latest day</span>
        <span>Refreshed</span>
      </div>
      {items.map((item) => (
        <div className="retention-row retention-row--analytics" key={item.table_name} role="row">
          <span>{item.table_name}</span>
          <span>{item.status}</span>
          <span>{formatNumber(item.rows)}</span>
          <span>{formatDateTime(item.latest_activity_date)}</span>
          <span>{formatAge(item.last_updated_age_seconds, item.last_updated_at)}</span>
        </div>
      ))}
    </div>
  );
}

function NotesPanel({ notes }: { notes: string[] }) {
  const uniqueNotes = Array.from(new Set(notes.filter(Boolean)));
  if (uniqueNotes.length === 0) {
    return null;
  }
  return (
    <section className="surface panel panel--wide" aria-label="Retention notes">
      <p className="eyebrow">Notes</p>
      {uniqueNotes.map((note) => (
        <p className="muted" key={note}>
          {note}
        </p>
      ))}
    </section>
  );
}

function SafeErrorPanel({ error }: { error: AdminApiError }) {
  return (
    <article className="surface panel panel--wide" role="alert">
      <p className="eyebrow">Safe error</p>
      <h3>{error.message}</h3>
      <p>Code: {error.code}</p>
    </article>
  );
}

function summarizeRetention(status?: OperatorRetentionStatusDTO) {
  if (!status) {
    return {
      conversationExpiredRows: 0,
      expiredRows: 0,
      jobEventExpiredRows: 0,
      orphanArtifacts: 0,
    };
  }
  return {
    conversationExpiredRows: sumRows(status.retention, "conversation"),
    expiredRows: status.retention.reduce((total, item) => total + item.expired_rows, 0),
    jobEventExpiredRows: sumRows(status.retention, "job_events"),
    orphanArtifacts: status.orphan_artifacts.total,
  };
}

function sumRows(items: OperatorRetentionTableDTO[], tableNamePart: string): number {
  return items
    .filter((item) => item.table_name.includes(tableNamePart))
    .reduce((total, item) => total + item.expired_rows, 0);
}

function formatDateTime(value?: string): string {
  if (!value) {
    return "n/a";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "n/a";
  }
  return date.toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

function formatAge(seconds: number, at?: string): string {
  if (!at) {
    return "n/a";
  }
  return `${formatDuration(seconds)} / ${formatDateTime(at)}`;
}

function formatDuration(seconds: number): string {
  if (seconds <= 0) {
    return "0s";
  }
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) {
    return `${days}d ${hours}h`;
  }
  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }
  return `${minutes}m`;
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) {
    return "0 B";
  }
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat().format(value);
}
