import { FormEvent, useEffect, useMemo, useState } from "react";
import type { AdminApiError, AdminClient } from "../api/adminClient";
import { toSafeAdminError } from "../api/adminClient";
import type { OperatorJobDetailDTO, OperatorJobsDTO } from "../api/jobs";

type JobsScreenProps = {
  adminTokenSet: boolean;
  client: AdminClient;
};

type JobFilters = {
  status: string;
  kind: string;
  errorClass: string;
  createdFrom: string;
  createdTo: string;
  correlationId: string;
};

type JobsState = {
  data?: OperatorJobsDTO;
  error?: AdminApiError;
  loading: boolean;
};

type DetailState = {
  data?: OperatorJobDetailDTO;
  error?: AdminApiError;
  loading: boolean;
};

const emptyFilters: JobFilters = {
  status: "",
  kind: "",
  errorClass: "",
  createdFrom: "",
  createdTo: "",
  correlationId: "",
};

const statusOptions = [
  "",
  "queued",
  "provider_processing",
  "postprocessing",
  "delivering",
  "succeeded",
  "failed_retryable",
  "failed_terminal",
  "awaiting_payment",
  "cancelled",
  "expired",
];

const kindOptions = [
  "",
  "text_generate",
  "image_generate",
  "image_edit",
  "video_generate",
  "video_image_to_video",
  "text",
  "image",
  "video",
];

export function JobsScreen({ adminTokenSet, client }: JobsScreenProps) {
  const [filters, setFilters] = useState<JobFilters>(emptyFilters);
  const [query, setQuery] = useState<JobFilters>(emptyFilters);
  const [jobs, setJobs] = useState<JobsState>({ loading: false });
  const [selectedLookupID, setSelectedLookupID] = useState("");
  const [detail, setDetail] = useState<DetailState>({ loading: false });

  const selectedDisplayID = useMemo(() => {
    const selected = jobs.data?.items.find((item) => item.lookup_id === selectedLookupID);
    return selected?.display_id ?? detail.data?.job.display_id ?? "";
  }, [detail.data?.job.display_id, jobs.data?.items, selectedLookupID]);

  useEffect(() => {
    if (!adminTokenSet) {
      setJobs({ loading: false });
      setSelectedLookupID("");
      return;
    }
    const controller = new AbortController();
    setJobs((current) => ({ data: current.data, loading: true }));
    client
      .request<OperatorJobsDTO>(operatorJobsPath(query), { signal: controller.signal })
      .then((data) => {
        setJobs({ data, loading: false });
        setSelectedLookupID((current) => {
          if (current && data.items.some((item) => item.lookup_id === current)) {
            return current;
          }
          return data.items[0]?.lookup_id ?? "";
        });
      })
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          setJobs({ error: toSafeAdminError(error), loading: false });
          setSelectedLookupID("");
        }
      });
    return () => controller.abort();
  }, [adminTokenSet, client, query]);

  useEffect(() => {
    if (!adminTokenSet || !selectedLookupID) {
      setDetail({ loading: false });
      return;
    }
    const controller = new AbortController();
    setDetail((current) => ({ data: current.data, loading: true }));
    const detailPath = `/admin/jobs/${encodeURIComponent(selectedLookupID)}/operator` as `/admin/${string}`;
    client
      .request<OperatorJobDetailDTO>(detailPath, {
        signal: controller.signal,
      })
      .then((data) => setDetail({ data, loading: false }))
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          setDetail({ error: toSafeAdminError(error), loading: false });
        }
      });
    return () => controller.abort();
  }, [adminTokenSet, client, selectedLookupID]);

  function submitFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setQuery(filters);
  }

  function resetFilters() {
    setFilters(emptyFilters);
    setQuery(emptyFilters);
  }

  if (!adminTokenSet) {
    return (
      <article className="surface panel panel--wide" role="status">
        <p className="eyebrow">Auth required</p>
        <h3>Jobs are locked</h3>
        <p>Enter an admin token to load read-only job and worker state.</p>
      </article>
    );
  }

  return (
    <div className="ops-stack">
      <form className="surface filters-panel" onSubmit={submitFilters}>
        <label>
          <span>Status</span>
          <select value={filters.status} onChange={(event) => setFilters({ ...filters, status: event.target.value })}>
            {statusOptions.map((status) => (
              <option key={status || "all"} value={status}>
                {status || "all"}
              </option>
            ))}
          </select>
        </label>
        <label>
          <span>Kind</span>
          <select value={filters.kind} onChange={(event) => setFilters({ ...filters, kind: event.target.value })}>
            {kindOptions.map((kind) => (
              <option key={kind || "all"} value={kind}>
                {kind || "all"}
              </option>
            ))}
          </select>
        </label>
        <label>
          <span>Error class</span>
          <input
            autoComplete="off"
            onChange={(event) => setFilters({ ...filters, errorClass: event.target.value })}
            placeholder="provider_timeout"
            value={filters.errorClass}
          />
        </label>
        <label>
          <span>Correlation</span>
          <input
            autoComplete="off"
            onChange={(event) => setFilters({ ...filters, correlationId: event.target.value })}
            placeholder="exact backend correlation id"
            value={filters.correlationId}
          />
        </label>
        <label>
          <span>Created from</span>
          <input
            onChange={(event) => setFilters({ ...filters, createdFrom: event.target.value })}
            type="datetime-local"
            value={filters.createdFrom}
          />
        </label>
        <label>
          <span>Created to</span>
          <input
            onChange={(event) => setFilters({ ...filters, createdTo: event.target.value })}
            type="datetime-local"
            value={filters.createdTo}
          />
        </label>
        <div className="filter-actions">
          <button type="submit">Apply</button>
          <button className="button-secondary" onClick={resetFilters} type="button">
            Reset
          </button>
        </div>
      </form>

      {jobs.error ? <SafeErrorPanel error={jobs.error} /> : null}
      {jobs.data ? <QueueSummary data={jobs.data} loading={jobs.loading} /> : null}

      <div className="jobs-layout">
        <section className="surface jobs-list" aria-label="Jobs list">
          <div className="section-heading">
            <div>
              <p className="eyebrow">Read-only jobs</p>
              <h3>{jobs.loading && !jobs.data ? "Loading jobs" : `${jobs.data?.pagination.count ?? 0} jobs shown`}</h3>
            </div>
            <span>{jobs.data?.pagination.has_more ? "more available" : "bounded page"}</span>
          </div>
          {jobs.data && jobs.data.items.length > 0 ? (
            <div className="jobs-table" role="table">
              <div className="jobs-row jobs-row--head" role="row">
                <span>ID</span>
                <span>Status</span>
                <span>Kind</span>
                <span>Error</span>
                <span>Age</span>
              </div>
              {jobs.data.items.map((item) => (
                <button
                  className={item.lookup_id === selectedLookupID ? "jobs-row jobs-row--active" : "jobs-row"}
                  key={item.lookup_id}
                  onClick={() => setSelectedLookupID(item.lookup_id)}
                  type="button"
                >
                  <span>{item.display_id}</span>
                  <span>{item.status}</span>
                  <span>{item.operation}</span>
                  <span>{item.error_class || "none"}</span>
                  <span>{formatDuration(item.age_seconds)}</span>
                </button>
              ))}
            </div>
          ) : (
            <p className="muted">{jobs.loading ? "Loading safe job rows." : "No jobs match the current filters."}</p>
          )}
        </section>

        <JobDetailPanel detail={detail} displayID={selectedDisplayID} />
      </div>
    </div>
  );
}

function QueueSummary({ data, loading }: { data: OperatorJobsDTO; loading: boolean }) {
  return (
    <section className="surface queue-panel" aria-label="Worker and queue summary">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Workers and queues</p>
          <h3>Degradation: {data.queue.degradation_state}</h3>
        </div>
        <span>{loading ? "Refreshing" : `Generated ${formatDateTime(data.queue.generated_at)}`}</span>
      </div>
      <div className="queue-metrics">
        {data.queue.backlog.map((metric) => (
          <div className={`queue-metric queue-metric--${metric.status}`} key={metric.label}>
            <span>{metric.label}</span>
            <strong>{metric.value}</strong>
          </div>
        ))}
        <div className="queue-metric queue-metric--not_wired">
          <span>oldest queued</span>
          <strong>{data.queue.oldest_queued_age_seconds === undefined ? "none" : formatDuration(data.queue.oldest_queued_age_seconds)}</strong>
        </div>
        <div className="queue-metric queue-metric--not_wired">
          <span>DLQ</span>
          <strong>{data.queue.dlq.status}</strong>
        </div>
        <div className="queue-metric queue-metric--not_wired">
          <span>provider circuit</span>
          <strong>{data.queue.provider_circuit.status}</strong>
        </div>
      </div>
      <p className="muted">{data.queue.dlq.reason}</p>
    </section>
  );
}

function JobDetailPanel({ detail, displayID }: { detail: DetailState; displayID: string }) {
  if (!displayID) {
    return (
      <aside className="surface detail-panel">
        <p className="eyebrow">Job detail</p>
        <h3>No job selected</h3>
        <p className="muted">Select a row to inspect safe job state.</p>
      </aside>
    );
  }
  if (detail.error) {
    return <SafeErrorPanel error={detail.error} />;
  }
  if (detail.loading && !detail.data) {
    return (
      <aside className="surface detail-panel" role="status">
        <p className="eyebrow">Loading</p>
        <h3>{displayID}</h3>
        <p className="muted">Loading safe detail.</p>
      </aside>
    );
  }
  if (!detail.data) {
    return (
      <aside className="surface detail-panel">
        <p className="eyebrow">Job detail</p>
        <h3>{displayID}</h3>
        <p className="muted">Detail is not loaded yet.</p>
      </aside>
    );
  }
  return (
    <aside className="surface detail-panel">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Job detail</p>
          <h3>{detail.data.job.display_id}</h3>
        </div>
        <span>{detail.loading ? "Refreshing" : detail.data.job.status}</span>
      </div>

      <dl className="detail-list">
        <div>
          <dt>Correlation</dt>
          <dd>{detail.data.job.correlation_ref || "none"}</dd>
        </div>
        <div>
          <dt>Operation</dt>
          <dd>{detail.data.job.operation}</dd>
        </div>
        <div>
          <dt>Reservation</dt>
          <dd>{detail.data.reservation ? `${detail.data.reservation.status} / ${detail.data.reservation.amount}` : "none"}</dd>
        </div>
        <div>
          <dt>Delivery</dt>
          <dd>{`${detail.data.delivery.status} / attempts ${detail.data.delivery.attempts}`}</dd>
        </div>
        <div>
          <dt>Retry count</dt>
          <dd>{detail.data.delivery.retry_count}</dd>
        </div>
        <div>
          <dt>Next statuses</dt>
          <dd>{detail.data.allowed_next_statuses.length > 0 ? detail.data.allowed_next_statuses.join(", ") : "terminal"}</dd>
        </div>
      </dl>

      <section className="detail-subsection">
        <h4>Artifacts</h4>
        <p className="muted">
          Inputs: {detail.data.artifacts.input_refs.join(", ") || "none"} · Outputs:{" "}
          {detail.data.artifacts.output_refs.join(", ") || "none"}
        </p>
      </section>

      <section className="detail-subsection">
        <h4>Delivery events</h4>
        {detail.data.delivery_events.length > 0 ? (
          <div className="event-list">
            {detail.data.delivery_events.map((event) => (
              <div key={`${event.type}:${event.attempt_no}:${event.status}`}>
                <strong>{`${event.type} / ${event.status}`}</strong>
                <span>{`attempt ${event.attempt_no}`}</span>
                <span>{event.error_class || "no error"}</span>
              </div>
            ))}
          </div>
        ) : (
          <p className="muted">No persisted delivery attempts.</p>
        )}
      </section>
    </aside>
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

function operatorJobsPath(filters: JobFilters): `/admin/${string}` {
  const params = new URLSearchParams({ limit: "20" });
  if (filters.status) {
    params.set("status", filters.status);
  }
  if (filters.kind) {
    params.set("kind", filters.kind);
  }
  if (filters.errorClass.trim()) {
    params.set("error_class", filters.errorClass.trim());
  }
  if (filters.correlationId.trim()) {
    params.set("correlation_id", filters.correlationId.trim());
  }
  const createdFrom = toISOString(filters.createdFrom);
  if (createdFrom) {
    params.set("created_from", createdFrom);
  }
  const createdTo = toISOString(filters.createdTo);
  if (createdTo) {
    params.set("created_to", createdTo);
  }
  return `/admin/jobs/operator?${params.toString()}`;
}

function toISOString(value: string): string {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toISOString();
}

function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "unknown";
  }
  return date.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) {
    return "unknown";
  }
  if (seconds < 60) {
    return `${Math.round(seconds)}s`;
  }
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes}m`;
  }
  const hours = Math.floor(minutes / 60);
  if (hours < 48) {
    return `${hours}h`;
  }
  return `${Math.floor(hours / 24)}d`;
}
