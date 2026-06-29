import { useEffect, useState } from "react";
import type { AdminApiError, AdminClient } from "../api/adminClient";
import { createIdempotencyKey, toSafeAdminError } from "../api/adminClient";
import type { OperatorDLQDTO, OperatorDLQItemDTO, OperatorDLQReplayResultDTO } from "../api/jobs";
import { OperatorConfirmDialog } from "../components/OperatorConfirmDialog";

type DLQScreenProps = {
  adminTokenSet: boolean;
  client: AdminClient;
};

type DLQState = {
  data?: OperatorDLQDTO;
  error?: AdminApiError;
  loading: boolean;
};

type ReplayState = {
  data?: OperatorDLQReplayResultDTO;
  error?: AdminApiError;
  loading: boolean;
};

type ReplayConfirmation =
  | {
      allowPaidProvider: boolean;
      displayID: string;
      jobID: string;
      mode: "single";
      target: string;
    }
  | {
      limit: number;
      mode: "batch";
    };

export function DLQScreen({ adminTokenSet, client }: DLQScreenProps) {
  const [cursor, setCursor] = useState("");
  const [cursorStack, setCursorStack] = useState<string[]>([]);
  const [dlq, setDLQ] = useState<DLQState>({ loading: false });
  const [replay, setReplay] = useState<ReplayState>({ loading: false });
  const [confirmation, setConfirmation] = useState<ReplayConfirmation>();
  const [refreshNonce, setRefreshNonce] = useState(0);

  useEffect(() => {
    if (!adminTokenSet) {
      setDLQ({ loading: false });
      return;
    }
    const controller = new AbortController();
    setDLQ((current) => ({ data: current.data, loading: true }));
    client
      .request<OperatorDLQDTO>(dlqPath(cursor), { signal: controller.signal })
      .then((data) => setDLQ({ data, loading: false }))
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          setDLQ({ error: toSafeAdminError(error), loading: false });
        }
      });
    return () => controller.abort();
  }, [adminTokenSet, client, cursor, refreshNonce]);

  function refresh() {
    setRefreshNonce((current) => current + 1);
  }

  function goNextPage() {
    const nextCursor = dlq.data?.pagination.next_cursor;
    if (!nextCursor) {
      return;
    }
    setCursorStack((current) => [...current, cursor]);
    setCursor(nextCursor);
  }

  function goPreviousPage() {
    if (cursorStack.length === 0) {
      return;
    }
    setCursor(cursorStack[cursorStack.length - 1] ?? "");
    setCursorStack((current) => current.slice(0, -1));
  }

  async function replaySingle(jobID: string, allowPaidProvider: boolean) {
    setReplay({ loading: true });
    const path = `/admin/jobs/${encodeURIComponent(jobID)}/replay` as `/admin/${string}`;
    try {
      const data = await client.request<OperatorDLQReplayResultDTO>(path, {
        method: "POST",
        body: { allow_paid_provider: allowPaidProvider },
        idempotencyKey: createIdempotencyKey("dlq_replay"),
      });
      setReplay({ data, loading: false });
      refresh();
    } catch (error) {
      setReplay({ error: toSafeAdminError(error), loading: false });
    }
  }

  async function replayBatch() {
    setReplay({ loading: true });
    try {
      const data = await client.request<OperatorDLQReplayResultDTO>("/admin/jobs/dlq/replay", {
        method: "POST",
        body: { limit: dlq.data?.replay.batch_limit ?? 25 },
        idempotencyKey: createIdempotencyKey("dlq_batch_replay"),
      });
      setReplay({ data, loading: false });
      refresh();
    } catch (error) {
      setReplay({ error: toSafeAdminError(error), loading: false });
    }
  }

  function requestBatchReplay() {
    setConfirmation({ limit: dlq.data?.replay.batch_limit ?? 25, mode: "batch" });
  }

  function requestSingleReplay(item: OperatorDLQItemDTO, allowPaidProvider: boolean) {
    setConfirmation({
      allowPaidProvider,
      displayID: item.job.display_id,
      jobID: item.job.lookup_id,
      mode: "single",
      target: item.replay_target,
    });
  }

  async function confirmReplay() {
    const draft = confirmation;
    if (!draft) {
      return;
    }
    if (draft.mode === "batch") {
      await replayBatch();
    } else {
      await replaySingle(draft.jobID, draft.allowPaidProvider);
    }
    setConfirmation(undefined);
  }

  if (!adminTokenSet) {
    return (
      <article className="surface panel panel--wide" role="status">
        <p className="eyebrow">Access required</p>
        <h3>DLQ is locked</h3>
        <p>Enter an admin token to inspect retryable failures.</p>
      </article>
    );
  }

  return (
    <div className="ops-stack">
      <section className="surface queue-panel" aria-label="DLQ replay policy">
        <div className="section-heading">
          <div>
            <p className="eyebrow">DLQ and retry tools</p>
            <h3>{dlq.loading && !dlq.data ? "Loading failed jobs" : `${dlq.data?.pagination.count ?? 0} jobs shown`}</h3>
          </div>
          <span>{dlq.data ? `Generated ${formatDateTime(dlq.data.generated_at)}` : "not loaded"}</span>
        </div>
        <div className="queue-metrics">
          <div className="queue-metric queue-metric--warning">
            <span>batch limit</span>
            <strong>{dlq.data?.replay.batch_limit ?? 25}</strong>
          </div>
          <div className="queue-metric queue-metric--warning">
            <span>batch provider jobs</span>
            <strong>{dlq.data?.replay.batch_skips_paid_provider ? "skipped" : "allowed"}</strong>
          </div>
          <div className="queue-metric queue-metric--ok">
            <span>single statuses</span>
            <strong>{dlq.data?.replay.single_allowed_statuses.join(", ") ?? "failed_retryable"}</strong>
          </div>
        </div>
        <p className="muted">
          Replay never exposes raw prompts, provider payloads, private URLs or user identifiers. Paid/provider jobs require single-job triage.
        </p>
        <div className="filter-actions">
          <button disabled={replay.loading || dlq.loading} onClick={requestBatchReplay} type="button">
            Replay safe batch
          </button>
          <button className="button-secondary" disabled={dlq.loading} onClick={refresh} type="button">
            Refresh
          </button>
        </div>
      </section>

      {dlq.error ? <SafeErrorPanel error={dlq.error} /> : null}
      {replay.error ? <SafeErrorPanel error={replay.error} /> : null}
      {replay.data ? <ReplayResult data={replay.data} loading={replay.loading} /> : null}

      <section className="surface jobs-list" aria-label="DLQ jobs">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Failed jobs</p>
            <h3>{dlq.data?.pagination.has_more ? "More failures available" : "Bounded page"}</h3>
          </div>
          <span>{dlq.loading ? "Refreshing" : "ready"}</span>
        </div>
        <div className="filter-actions">
          <button className="button-secondary" disabled={cursorStack.length === 0 || dlq.loading} onClick={goPreviousPage} type="button">
            Previous
          </button>
          <button className="button-secondary" disabled={!dlq.data?.pagination.next_cursor || dlq.loading} onClick={goNextPage} type="button">
            Next
          </button>
        </div>
        {dlq.data && dlq.data.items.length > 0 ? (
          <div className="jobs-table" role="table">
            <div className="jobs-row jobs-row--head" role="row">
              <span>ID</span>
              <span>Status</span>
              <span>Error</span>
              <span>Attempts</span>
              <span>Replay</span>
            </div>
            {dlq.data.items.map((item) => (
              <div className="jobs-row" key={item.job.lookup_id} role="row">
                <span>{item.job.display_id}</span>
                <span>{item.job.status}</span>
                <span>{item.last_error_class || item.job.error_class || "none"}</span>
                <span>{`${item.attempt_count} / provider ${item.provider_task_count}`}</span>
                <span>
                  {item.safe_replay ? (
                    <button disabled={replay.loading} onClick={() => requestSingleReplay(item, false)} type="button">
                      Replay
                    </button>
                  ) : (
                    <button
                      className="button-secondary"
                      disabled={replay.loading || item.job.cost_captured > 0}
                      onClick={() => requestSingleReplay(item, true)}
                      type="button"
                    >
                      Override
                    </button>
                  )}
                </span>
                <span className="muted">{item.replay_blocked_reason || `target ${item.replay_target}`}</span>
              </div>
            ))}
          </div>
        ) : (
          <p className="muted">{dlq.loading ? "Loading DLQ rows." : "No failed jobs match the DLQ filter."}</p>
        )}
      </section>
      {confirmation ? (
        <ReplayConfirmationDialog
          busy={replay.loading}
          confirmation={confirmation}
          onCancel={() => setConfirmation(undefined)}
          onConfirm={confirmReplay}
        />
      ) : null}
    </div>
  );
}

function ReplayConfirmationDialog({
  busy,
  confirmation,
  onCancel,
  onConfirm,
}: {
  busy: boolean;
  confirmation: ReplayConfirmation;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  if (confirmation.mode === "batch") {
    return (
      <OperatorConfirmDialog
        busy={busy}
        confirmLabel="Replay batch"
        description="Backend will replay only safe retryable jobs from the bounded DLQ page. Paid/provider jobs stay skipped by policy."
        onCancel={onCancel}
        onConfirm={onConfirm}
        title="Replay safe DLQ batch"
      >
        <dl className="detail-list">
          <div>
            <dt>Limit</dt>
            <dd>{confirmation.limit}</dd>
          </div>
          <div>
            <dt>Scope</dt>
            <dd>failed_retryable jobs only</dd>
          </div>
        </dl>
      </OperatorConfirmDialog>
    );
  }
  return (
    <OperatorConfirmDialog
      busy={busy}
      confirmLabel={confirmation.allowPaidProvider ? "Override replay" : "Replay job"}
      danger={confirmation.allowPaidProvider}
      description={
        confirmation.allowPaidProvider
          ? "This single-job override can consume provider quota. Use it only after checking payment and delivery state."
          : "Backend will requeue this failed retryable job through the same worker path as normal job intake."
      }
      onCancel={onCancel}
      onConfirm={onConfirm}
      title={confirmation.allowPaidProvider ? "Override provider replay" : "Replay failed job"}
    >
      <dl className="detail-list">
        <div>
          <dt>Job</dt>
          <dd>{confirmation.displayID}</dd>
        </div>
        <div>
          <dt>Target</dt>
          <dd>{confirmation.target}</dd>
        </div>
      </dl>
    </OperatorConfirmDialog>
  );
}

function ReplayResult({ data, loading }: { data: OperatorDLQReplayResultDTO; loading: boolean }) {
  return (
    <section className="surface queue-panel" aria-label="Replay result">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Replay result</p>
          <h3>{loading ? "Replay running" : `${data.replayed.length} replayed / ${data.skipped.length} skipped`}</h3>
        </div>
        <span>{formatDateTime(data.generated_at)}</span>
      </div>
      <div className="event-list">
        {[...data.replayed, ...data.skipped].map((item) => (
          <div key={`${item.lookup_id}:${item.result}`}>
            <strong>{`${item.display_id} / ${item.result}`}</strong>
            <span>{item.status}</span>
            <span>{item.reason || "ok"}</span>
          </div>
        ))}
      </div>
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

function dlqPath(cursor: string): `/admin/${string}` {
  const params = new URLSearchParams({ limit: "25", status: "failed_retryable" });
  if (cursor) {
    params.set("cursor", cursor);
  }
  return `/admin/jobs/dlq?${params.toString()}`;
}

function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "unknown";
  }
  return date.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}
