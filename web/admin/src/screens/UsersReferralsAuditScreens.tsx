import { FormEvent, useEffect, useState } from "react";
import type { AdminApiError, AdminClient } from "../api/adminClient";
import { toSafeAdminError } from "../api/adminClient";
import type {
  OperatorAuditEntryDTO,
  OperatorAuditLogDTO,
  OperatorReferralsDTO,
  OperatorUserRecentJobDTO,
  OperatorUsersDTO,
  SuspiciousReferralDTO,
} from "../api/usersReferralsAudit";

type OperatorScreenProps = {
  adminTokenSet: boolean;
  client: AdminClient;
};

type LoadState<T> = {
  data?: T;
  error?: AdminApiError;
  loading: boolean;
};

type UserFilters = {
  userId: string;
};

type ReferralFilters = {
  code: string;
  minRegistered: string;
  minTotal: string;
};

type AuditFilters = {
  action: string;
  targetType: string;
  result: string;
};

const emptyUserFilters: UserFilters = { userId: "" };
const emptyReferralFilters: ReferralFilters = { code: "", minRegistered: "10", minTotal: "50" };
const emptyAuditFilters: AuditFilters = { action: "", targetType: "", result: "" };

export function UsersScreen({ adminTokenSet, client }: OperatorScreenProps) {
  const [filters, setFilters] = useState<UserFilters>(emptyUserFilters);
  const [query, setQuery] = useState<UserFilters>(emptyUserFilters);
  const state = useOperatorData<OperatorUsersDTO>(adminTokenSet, client, usersPath(query));

  function submitFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setQuery(filters);
  }

  function resetFilters() {
    setFilters(emptyUserFilters);
    setQuery(emptyUserFilters);
  }

  if (!adminTokenSet) {
    return <LockedPanel title="Users are locked" text="Enter an admin token to load safe user summaries." />;
  }

  return (
    <div className="ops-stack">
      <form className="surface filters-panel" onSubmit={submitFilters}>
        <label>
          <span>User lookup</span>
          <input
            autoComplete="off"
            onChange={(event) => setFilters({ userId: event.target.value })}
            placeholder="internal user id"
            value={filters.userId}
          />
        </label>
        <div className="filter-actions">
          <button type="submit">Apply</button>
          <button className="button-secondary" onClick={resetFilters} type="button">
            Reset
          </button>
        </div>
      </form>

      {state.error ? <SafeErrorPanel error={state.error} /> : null}
      {!state.data ? <LoadingPanel loading={state.loading} title="Loading user console" /> : null}
      {state.data ? <UserConsole data={state.data} loading={state.loading} /> : null}
    </div>
  );
}

export function ReferralsScreen({ adminTokenSet, client }: OperatorScreenProps) {
  const [filters, setFilters] = useState<ReferralFilters>(emptyReferralFilters);
  const [query, setQuery] = useState<ReferralFilters>(emptyReferralFilters);
  const state = useOperatorData<OperatorReferralsDTO>(adminTokenSet, client, referralsPath(query));

  function submitFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setQuery(filters);
  }

  function resetFilters() {
    setFilters(emptyReferralFilters);
    setQuery(emptyReferralFilters);
  }

  if (!adminTokenSet) {
    return <LockedPanel title="Referrals are locked" text="Enter an admin token to load aggregate referral signals." />;
  }

  return (
    <div className="ops-stack">
      <form className="surface filters-panel" onSubmit={submitFilters}>
        <label>
          <span>Code</span>
          <input
            autoComplete="off"
            onChange={(event) => setFilters({ ...filters, code: event.target.value })}
            placeholder="optional public code"
            value={filters.code}
          />
        </label>
        <label>
          <span>Min registered</span>
          <input
            min="1"
            onChange={(event) => setFilters({ ...filters, minRegistered: event.target.value })}
            type="number"
            value={filters.minRegistered}
          />
        </label>
        <label>
          <span>Min total</span>
          <input
            min="1"
            onChange={(event) => setFilters({ ...filters, minTotal: event.target.value })}
            type="number"
            value={filters.minTotal}
          />
        </label>
        <div className="filter-actions">
          <button type="submit">Apply</button>
          <button className="button-secondary" onClick={resetFilters} type="button">
            Reset
          </button>
        </div>
      </form>

      {state.error ? <SafeErrorPanel error={state.error} /> : null}
      {!state.data ? <LoadingPanel loading={state.loading} title="Loading referral console" /> : null}
      {state.data ? <ReferralConsole data={state.data} loading={state.loading} /> : null}
    </div>
  );
}

export function AuditLogScreen({ adminTokenSet, client }: OperatorScreenProps) {
  const [filters, setFilters] = useState<AuditFilters>(emptyAuditFilters);
  const [query, setQuery] = useState<AuditFilters>(emptyAuditFilters);
  const state = useOperatorData<OperatorAuditLogDTO>(adminTokenSet, client, auditPath(query));

  function submitFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setQuery(filters);
  }

  function resetFilters() {
    setFilters(emptyAuditFilters);
    setQuery(emptyAuditFilters);
  }

  if (!adminTokenSet) {
    return <LockedPanel title="Audit log is locked" text="Enter an admin token to load sanitized operator audit rows." />;
  }

  return (
    <div className="ops-stack">
      <form className="surface filters-panel" onSubmit={submitFilters}>
        <label>
          <span>Action</span>
          <input
            autoComplete="off"
            onChange={(event) => setFilters({ ...filters, action: event.target.value })}
            placeholder="admin_operator_jobs_list"
            value={filters.action}
          />
        </label>
        <label>
          <span>Target type</span>
          <input
            autoComplete="off"
            onChange={(event) => setFilters({ ...filters, targetType: event.target.value })}
            placeholder="jobs"
            value={filters.targetType}
          />
        </label>
        <label>
          <span>Result</span>
          <select value={filters.result} onChange={(event) => setFilters({ ...filters, result: event.target.value })}>
            <option value="">all</option>
            <option value="success">success</option>
            <option value="error">error</option>
          </select>
        </label>
        <div className="filter-actions">
          <button type="submit">Apply</button>
          <button className="button-secondary" onClick={resetFilters} type="button">
            Reset
          </button>
        </div>
      </form>

      {state.error ? <SafeErrorPanel error={state.error} /> : null}
      {!state.data ? <LoadingPanel loading={state.loading} title="Loading audit log" /> : null}
      {state.data ? <AuditConsole data={state.data} loading={state.loading} /> : null}
    </div>
  );
}

function useOperatorData<T>(adminTokenSet: boolean, client: AdminClient, path: `/admin/${string}`): LoadState<T> {
  const [state, setState] = useState<LoadState<T>>({ loading: false });

  useEffect(() => {
    if (!adminTokenSet) {
      setState({ loading: false });
      return;
    }
    const controller = new AbortController();
    setState((current) => ({ data: current.data, loading: true }));
    client
      .request<T>(path, { signal: controller.signal })
      .then((data) => setState({ data, loading: false }))
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          setState({ error: toSafeAdminError(error), loading: false });
        }
      });
    return () => controller.abort();
  }, [adminTokenSet, client, path]);

  return state;
}

function UserConsole({ data, loading }: { data: OperatorUsersDTO; loading: boolean }) {
  if (!data.user) {
    return (
      <>
        <section className="surface panel panel--wide" role="status">
          <p className="eyebrow">Safe lookup</p>
          <h3>User not selected</h3>
          <p>Use a protected internal lookup id to load one user summary.</p>
        </section>
        <NotesPanel notes={data.notes} />
      </>
    );
  }

  const jobMetrics = [
    { label: "total", value: data.user.jobs.total },
    { label: "active", value: data.user.jobs.active },
    { label: "succeeded", value: data.user.jobs.succeeded },
    { label: "failed", value: data.user.jobs.failed },
    { label: "text", value: data.user.jobs.text_jobs },
    { label: "image", value: data.user.jobs.image_jobs },
    { label: "video", value: data.user.jobs.video_jobs },
  ];
  const paymentMetrics = [
    { label: "total", value: String(data.payment.total) },
    { label: "pending", value: String(data.payment.pending) },
    { label: "succeeded", value: String(data.payment.succeeded) },
    { label: "failed", value: String(data.payment.failed) },
    { label: "refunded", value: String(data.payment.refunded) },
    { label: "credits", value: String(data.payment.credits_purchased) },
  ];
  return (
    <>
      <section className="surface queue-panel" aria-label="User summary">
        <div className="section-heading">
          <div>
            <p className="eyebrow">User summary</p>
            <h3>{data.user.user_ref}</h3>
          </div>
          <span>{loading ? "Refreshing" : `Generated ${formatDateTime(data.generated_at)}`}</span>
        </div>
        <dl className="detail-list">
          <div>
            <dt>Role / status</dt>
            <dd>{`${data.user.role} / ${data.user.status}`}</dd>
          </div>
          <div>
            <dt>Risk</dt>
            <dd>{data.user.risk_class}</dd>
          </div>
          <div>
            <dt>Locale</dt>
            <dd>{data.user.locale || "unknown"}</dd>
          </div>
          <div>
            <dt>Seen</dt>
            <dd>{`${formatDateTime(data.user.first_seen_at)} / ${formatDateTime(data.user.last_seen_at)}`}</dd>
          </div>
        </dl>
      </section>

      <section className="surface queue-panel" aria-label="User jobs and payments">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Activity</p>
            <h3>Jobs and payments</h3>
          </div>
          <span>bounded counts</span>
        </div>
        <div className="config-grid">
          {[...jobMetrics, ...paymentMetrics].map((metric) => (
            <div className="queue-metric" key={`${metric.label}:${metric.value}`}>
              <span>{metric.label}</span>
              <strong>{metric.value}</strong>
            </div>
          ))}
        </div>
      </section>

      <section className="surface queue-panel" aria-label="User referrals">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Referrals</p>
            <h3>{data.referrals.code || "no code"}</h3>
          </div>
          <span>{data.referrals.status}</span>
        </div>
        <div className="queue-metrics">
          {[
            { label: "invited", value: data.referrals.invited },
            { label: "registered", value: data.referrals.registered },
            { label: "activated", value: data.referrals.activated },
            { label: "rewarded", value: data.referrals.rewarded },
          ].map((metric) => (
            <div className="queue-metric" key={metric.label}>
              <span>{metric.label}</span>
              <strong>{metric.value}</strong>
            </div>
          ))}
        </div>
        <p className="muted">
          {data.referrals.invited_by
            ? `Invited by ${data.referrals.invited_by.source} / ${data.referrals.invited_by.status}`
            : "No invited-by relation."}
        </p>
      </section>

      <section className="surface jobs-list" aria-label="Recent user jobs">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Recent jobs</p>
            <h3>{`${data.recent_jobs?.length ?? 0} shown`}</h3>
          </div>
          <span>safe refs only</span>
        </div>
        <RecentJobsList items={data.recent_jobs ?? []} />
      </section>
      <NotesPanel notes={data.notes} />
    </>
  );
}

function ReferralConsole({ data, loading }: { data: OperatorReferralsDTO; loading: boolean }) {
  const distribution = [
    { label: "total", value: data.distribution.total },
    { label: "registered", value: data.distribution.registered_count },
    { label: "activated", value: data.distribution.activated_count },
    { label: "rewarded", value: data.distribution.rewarded_count },
  ];
  return (
    <>
      <section className="surface queue-panel" aria-label="Referral distribution">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Referral funnel</p>
            <h3>Global status distribution</h3>
          </div>
          <span>{loading ? "Refreshing" : `Generated ${formatDateTime(data.generated_at)}`}</span>
        </div>
        <div className="queue-metrics">
          {distribution.map((metric) => (
            <div className="queue-metric" key={metric.label}>
              <span>{metric.label}</span>
              <strong>{metric.value}</strong>
            </div>
          ))}
        </div>
      </section>

      {data.code_stats ? <CodeStatsPanel stats={data.code_stats} /> : null}

      <section className="surface jobs-list" aria-label="Suspicious referral codes">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Suspicious activity</p>
            <h3>{`${data.suspicious.length} codes shown`}</h3>
          </div>
          <span>{`registered ${data.suspicious_criteria.min_registered} / total ${data.suspicious_criteria.min_total}`}</span>
        </div>
        <SuspiciousReferralList items={data.suspicious} />
      </section>
      <NotesPanel notes={data.notes} />
    </>
  );
}

function AuditConsole({ data, loading }: { data: OperatorAuditLogDTO; loading: boolean }) {
  return (
    <>
      <section className="surface jobs-list" aria-label="Operator audit log">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Audit log</p>
            <h3>{`${data.pagination.count} entries shown`}</h3>
          </div>
          <span>{loading ? "Refreshing" : `Generated ${formatDateTime(data.generated_at)}`}</span>
        </div>
        <AuditList items={data.items} />
      </section>
      <NotesPanel notes={data.notes} />
    </>
  );
}

function RecentJobsList({ items }: { items: OperatorUserRecentJobDTO[] }) {
  if (items.length === 0) {
    return <p className="muted">No recent jobs in this bounded page.</p>;
  }
  return (
    <div className="event-list">
      {items.map((item) => (
        <div key={item.display_id}>
          <strong>{`${item.display_id} / ${item.status}`}</strong>
          <span>{`${item.operation} / ${item.modality}`}</span>
          <span>{item.error_class || "no error"}</span>
          <span>{`age ${formatDuration(item.age_seconds)}`}</span>
        </div>
      ))}
    </div>
  );
}

function CodeStatsPanel({ stats }: { stats: OperatorReferralsDTO["code_stats"] }) {
  if (!stats) {
    return null;
  }
  return (
    <section className="surface queue-panel" aria-label="Referral code stats">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Code stats</p>
          <h3>{stats.code}</h3>
        </div>
        <span>aggregate only</span>
      </div>
      <div className="queue-metrics">
        {[
          { label: "invited", value: stats.invited_count },
          { label: "registered", value: stats.registered_count },
          { label: "activated", value: stats.activated_count },
          { label: "rewarded", value: stats.rewarded_count },
        ].map((metric) => (
          <div className="queue-metric" key={metric.label}>
            <span>{metric.label}</span>
            <strong>{metric.value}</strong>
          </div>
        ))}
      </div>
    </section>
  );
}

function SuspiciousReferralList({ items }: { items: SuspiciousReferralDTO[] }) {
  if (items.length === 0) {
    return <p className="muted">No suspicious referral codes in this bounded page.</p>;
  }
  return (
    <div className="event-list">
      {items.map((item) => (
        <div key={item.code}>
          <strong>{`${item.code} / ${item.invited_count} invited`}</strong>
          <span>{`registered ${item.registered_count}, activated ${item.activated_count}, rewarded ${item.rewarded_count}`}</span>
          <span>{item.reasons.join(", ")}</span>
        </div>
      ))}
    </div>
  );
}

function AuditList({ items }: { items: OperatorAuditEntryDTO[] }) {
  if (items.length === 0) {
    return <p className="muted">No audit rows in this bounded page.</p>;
  }
  return (
    <div className="provider-table" role="table">
      <div className="audit-row audit-row--head" role="row">
        <span>Time</span>
        <span>Actor</span>
        <span>Action</span>
        <span>Target</span>
        <span>Result</span>
        <span>Request</span>
      </div>
      {items.map((item) => (
        <div className="audit-row" key={item.display_id} role="row">
          <span>{formatDateTime(item.created_at)}</span>
          <span>{item.actor_ref}</span>
          <span>{item.action}</span>
          <span>{`${item.target_type} / ${item.target_ref || "none"}`}</span>
          <span className={`status-pill status-pill--${item.result === "success" ? "ok" : "warning"}`}>{item.result}</span>
          <span>{item.request_ref || "none"}</span>
        </div>
      ))}
    </div>
  );
}

function NotesPanel({ notes }: { notes?: string[] }) {
  if (!notes || notes.length === 0) {
    return null;
  }
  return (
    <section className="surface panel panel--wide" aria-label="Operator notes">
      <p className="eyebrow">Notes</p>
      {notes.map((note) => (
        <p className="muted" key={note}>
          {note}
        </p>
      ))}
    </section>
  );
}

function LockedPanel({ title, text }: { title: string; text: string }) {
  return (
    <article className="surface panel panel--wide" role="status">
      <p className="eyebrow">Auth required</p>
      <h3>{title}</h3>
      <p>{text}</p>
    </article>
  );
}

function LoadingPanel({ loading, title }: { loading: boolean; title: string }) {
  return (
    <article className="surface panel panel--wide" role="status">
      <p className="eyebrow">{loading ? "Loading" : "No data"}</p>
      <h3>{title}</h3>
      <p>{loading ? "Requesting a bounded safe operator snapshot." : "The admin API has not returned data yet."}</p>
    </article>
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

function usersPath(filters: UserFilters): `/admin/${string}` {
  const params = new URLSearchParams();
  if (filters.userId.trim()) {
    params.set("user_id", filters.userId.trim());
  }
  const query = params.toString();
  return query ? `/admin/users/operator?${query}` : "/admin/users/operator";
}

function referralsPath(filters: ReferralFilters): `/admin/${string}` {
  const params = new URLSearchParams({ limit: "20" });
  if (filters.code.trim()) {
    params.set("code", filters.code.trim());
  }
  if (positiveNumber(filters.minRegistered)) {
    params.set("min_registered", filters.minRegistered);
  }
  if (positiveNumber(filters.minTotal)) {
    params.set("min_total", filters.minTotal);
  }
  return `/admin/referrals/operator?${params.toString()}`;
}

function auditPath(filters: AuditFilters): `/admin/${string}` {
  const params = new URLSearchParams({ limit: "30" });
  if (filters.action.trim()) {
    params.set("action", filters.action.trim());
  }
  if (filters.targetType.trim()) {
    params.set("target_type", filters.targetType.trim());
  }
  if (filters.result) {
    params.set("result", filters.result);
  }
  return `/admin/audit/operator?${params.toString()}`;
}

function positiveNumber(value: string): boolean {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0;
}

function formatDateTime(value?: string): string {
  if (!value) {
    return "unknown";
  }
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
  return `${hours}h`;
}
