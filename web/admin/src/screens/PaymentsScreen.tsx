import { FormEvent, useEffect, useState } from "react";
import type { AdminApiError, AdminClient } from "../api/adminClient";
import { toSafeAdminError } from "../api/adminClient";
import type {
  OperatorBillingDTO,
  OperatorPaymentsConsoleDTO,
  OperatorPaymentEventDTO,
  OperatorPaymentIntentDTO,
  OperatorPaymentRefundDTO,
} from "../api/payments";

type PaymentsScreenProps = {
  adminTokenSet: boolean;
  client: AdminClient;
};

type PaymentFilters = {
  status: string;
  provider: string;
  userId: string;
  staleAfterSeconds: string;
};

type PaymentsState = {
  data?: OperatorPaymentsConsoleDTO;
  error?: AdminApiError;
  loading: boolean;
};

const emptyFilters: PaymentFilters = {
  status: "",
  provider: "",
  userId: "",
  staleAfterSeconds: "300",
};

const statusOptions = [
  "",
  "created",
  "provider_pending",
  "waiting_for_user",
  "succeeded",
  "canceled",
  "expired",
  "failed",
  "refunded",
  "partially_refunded",
];

const providerOptions = ["", "mock", "yookassa"];

export function PaymentsScreen({ adminTokenSet, client }: PaymentsScreenProps) {
  const [filters, setFilters] = useState<PaymentFilters>(emptyFilters);
  const [query, setQuery] = useState<PaymentFilters>(emptyFilters);
  const [payments, setPayments] = useState<PaymentsState>({ loading: false });

  useEffect(() => {
    if (!adminTokenSet) {
      setPayments({ loading: false });
      return;
    }
    const controller = new AbortController();
    setPayments((current) => ({ data: current.data, loading: true }));
    client
      .request<OperatorPaymentsConsoleDTO>(operatorPaymentsPath(query), { signal: controller.signal })
      .then((data) => setPayments({ data, loading: false }))
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          setPayments({ error: toSafeAdminError(error), loading: false });
        }
      });
    return () => controller.abort();
  }, [adminTokenSet, client, query]);

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
        <h3>Payments are locked</h3>
        <p>Enter an admin token to load read-only payment and billing state.</p>
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
          <span>Provider</span>
          <select value={filters.provider} onChange={(event) => setFilters({ ...filters, provider: event.target.value })}>
            {providerOptions.map((provider) => (
              <option key={provider || "all"} value={provider}>
                {provider || "all"}
              </option>
            ))}
          </select>
        </label>
        <label>
          <span>User lookup</span>
          <input
            autoComplete="off"
            onChange={(event) => setFilters({ ...filters, userId: event.target.value })}
            placeholder="internal user id for billing snapshot"
            value={filters.userId}
          />
        </label>
        <label>
          <span>Stale after</span>
          <input
            min="1"
            onChange={(event) => setFilters({ ...filters, staleAfterSeconds: event.target.value })}
            type="number"
            value={filters.staleAfterSeconds}
          />
        </label>
        <div className="filter-actions">
          <button type="submit">Apply</button>
          <button className="button-secondary" onClick={resetFilters} type="button">
            Reset
          </button>
        </div>
      </form>

      {payments.error ? <SafeErrorPanel error={payments.error} /> : null}
      {payments.data ? <PaymentReconciliationPanel data={payments.data} loading={payments.loading} /> : null}
      {payments.loading && !payments.data ? (
        <article className="surface panel panel--wide" role="status">
          <p className="eyebrow">Loading</p>
          <h3>Loading payment console</h3>
          <p>Requesting safe read-only billing summaries.</p>
        </article>
      ) : null}

      {payments.data ? (
        <div className="payments-layout">
          <section className="surface jobs-list" aria-label="Payment intents">
            <div className="section-heading">
              <div>
                <p className="eyebrow">Payment intents</p>
                <h3>{`${payments.data.pagination.count} intents shown`}</h3>
              </div>
              <span>{payments.data.pagination.has_more ? "more available" : "bounded page"}</span>
            </div>
            <PaymentIntentsTable items={payments.data.intents} />
          </section>

          <BillingSnapshot billing={payments.data.billing} userLookupSet={Boolean(query.userId.trim())} />

          <section className="surface jobs-list" aria-label="Webhook provider events">
            <div className="section-heading">
              <div>
                <p className="eyebrow">Webhook inbox</p>
                <h3>{`${payments.data.events.length} events shown`}</h3>
              </div>
              <span>raw payloads omitted</span>
            </div>
            <PaymentEventsList items={payments.data.events} />
          </section>

          <section className="surface jobs-list" aria-label="Refund state">
            <div className="section-heading">
              <div>
                <p className="eyebrow">Refunds</p>
                <h3>{`${payments.data.refunds.length} refunds shown`}</h3>
              </div>
              <span>read-only</span>
            </div>
            <PaymentRefundsList items={payments.data.refunds} />
          </section>
        </div>
      ) : null}
    </div>
  );
}

function PaymentReconciliationPanel({ data, loading }: { data: OperatorPaymentsConsoleDTO; loading: boolean }) {
  const metrics = [
    { label: "status", value: data.reconciliation.status },
    { label: "pending", value: String(data.reconciliation.pending_count) },
    { label: "stale", value: String(data.reconciliation.stale_count) },
    { label: "unprocessed events", value: String(data.reconciliation.unprocessed_event_count) },
    { label: "refund rows", value: String(data.reconciliation.refund_count) },
    { label: "stale after", value: formatDuration(data.reconciliation.stale_after_seconds) },
  ];
  return (
    <section className="surface queue-panel" aria-label="Payment reconciliation">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Reconciliation</p>
          <h3>{data.reconciliation.status}</h3>
        </div>
        <span>{loading ? "Refreshing" : `Generated ${formatDateTime(data.generated_at)}`}</span>
      </div>
      <div className="queue-metrics">
        {metrics.map((metric) => (
          <div
            className={`queue-metric ${
              metric.value === "0" || metric.value === "ok" ? "queue-metric--ok" : "queue-metric--warning"
            }`}
            key={metric.label}
          >
            <span>{metric.label}</span>
            <strong>{metric.value}</strong>
          </div>
        ))}
      </div>
      <p className="muted">Payment state is backend-derived; redirects and provider payloads are not trusted here.</p>
    </section>
  );
}

function PaymentIntentsTable({ items }: { items: OperatorPaymentIntentDTO[] }) {
  if (items.length === 0) {
    return <p className="muted">No intents match the current filters.</p>;
  }
  return (
    <div className="payments-table" role="table">
      <div className="payments-row payments-row--head" role="row">
        <span>ID</span>
        <span>Status</span>
        <span>Provider</span>
        <span>Amount</span>
        <span>Capture</span>
        <span>Refund</span>
      </div>
      {items.map((item) => (
        <div className={item.stale ? "payments-row payments-row--warning" : "payments-row"} key={item.display_id} role="row">
          <span>{item.display_id}</span>
          <span>{item.status}</span>
          <span>{item.provider}</span>
          <span>{formatMoney(item.amount, item.currency)}</span>
          <span>{item.capture_state}</span>
          <span>{item.refund_state}</span>
        </div>
      ))}
    </div>
  );
}

function BillingSnapshot({ billing, userLookupSet }: { billing?: OperatorBillingDTO; userLookupSet: boolean }) {
  if (!userLookupSet) {
    return (
      <aside className="surface detail-panel">
        <p className="eyebrow">Billing snapshot</p>
        <h3>User lookup not set</h3>
        <p className="muted">Enter an internal user id to load ledger and reservation state.</p>
      </aside>
    );
  }
  if (!billing) {
    return (
      <aside className="surface detail-panel">
        <p className="eyebrow">Billing snapshot</p>
        <h3>No credit account</h3>
        <p className="muted">No ledger-backed credits account exists for this user yet.</p>
      </aside>
    );
  }
  return (
    <aside className="surface detail-panel">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Billing snapshot</p>
          <h3>{billing.user_ref}</h3>
        </div>
        <span>{`${billing.balance_credits} credits`}</span>
      </div>
      <section className="detail-subsection">
        <h4>Ledger</h4>
        {billing.ledger.length > 0 ? (
          <div className="event-list">
            {billing.ledger.map((entry) => (
              <div key={entry.display_id}>
                <strong>{`${entry.type} / ${entry.status}`}</strong>
                <span>{`${entry.amount} credits`}</span>
                <span>{entry.reason_class || "no reason class"}</span>
              </div>
            ))}
          </div>
        ) : (
          <p className="muted">No ledger entries.</p>
        )}
      </section>
      <section className="detail-subsection">
        <h4>Reservations</h4>
        {billing.reservations.length > 0 ? (
          <div className="event-list">
            {billing.reservations.map((reservation) => (
              <div key={reservation.display_id}>
                <strong>{`${reservation.status} / ${reservation.amount}`}</strong>
                <span>{reservation.job_ref}</span>
                <span>{`expires ${formatDateTime(reservation.expires_at)}`}</span>
              </div>
            ))}
          </div>
        ) : (
          <p className="muted">No reservations referenced by recent ledger entries.</p>
        )}
      </section>
    </aside>
  );
}

function PaymentEventsList({ items }: { items: OperatorPaymentEventDTO[] }) {
  if (items.length === 0) {
    return <p className="muted">No provider webhook events in this bounded page.</p>;
  }
  return (
    <div className="event-list">
      {items.map((item) => (
        <div key={item.display_id}>
          <strong>{`${item.event_type} / ${item.processed ? "processed" : "unprocessed"}`}</strong>
          <span>{`${item.provider} / ${item.provider_payment_ref || item.provider_refund_ref || "no provider ref"}`}</span>
          <span>{`received ${formatDateTime(item.received_at)}`}</span>
        </div>
      ))}
    </div>
  );
}

function PaymentRefundsList({ items }: { items: OperatorPaymentRefundDTO[] }) {
  if (items.length === 0) {
    return <p className="muted">No refund rows in this bounded page.</p>;
  }
  return (
    <div className="event-list">
      {items.map((item) => (
        <div key={item.display_id}>
          <strong>{`${item.status} / ${formatMinor(item.amount)}`}</strong>
          <span>{item.provider_refund_ref || "no provider refund ref"}</span>
          <span>{item.reason_present ? "reason recorded" : "no reason recorded"}</span>
        </div>
      ))}
    </div>
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

function operatorPaymentsPath(filters: PaymentFilters): `/billing/${string}` {
  const params = new URLSearchParams({ limit: "20" });
  if (filters.status) {
    params.set("status", filters.status);
  }
  if (filters.provider) {
    params.set("provider", filters.provider);
  }
  if (filters.userId.trim()) {
    params.set("user_id", filters.userId.trim());
  }
  const seconds = Number(filters.staleAfterSeconds);
  if (Number.isFinite(seconds) && seconds > 0) {
    params.set("stale_after", `${Math.round(seconds)}s`);
  }
  return `/billing/operator/console?${params.toString()}`;
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
  return `${hours}h`;
}

function formatMoney(amount: number, currency: string): string {
  return `${formatMinor(amount)} ${currency}`;
}

function formatMinor(amount: number): string {
  if (!Number.isFinite(amount)) {
    return "unknown";
  }
  return String(amount);
}
