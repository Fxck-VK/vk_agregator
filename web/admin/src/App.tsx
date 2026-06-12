import { Component, FormEvent, ReactNode, useEffect, useMemo, useState } from "react";
import { AdminApiError, createAdminClient, toSafeAdminError } from "./api/adminClient";
import type { OverviewCardDTO, OverviewDTO } from "./api/overview";

type ScreenId =
  | "overview"
  | "jobs"
  | "users"
  | "payments"
  | "providers"
  | "media"
  | "referrals"
  | "alerts"
  | "audit"
  | "config";

type Screen = {
  id: ScreenId;
  title: string;
  eyebrow: string;
  summary: string;
  panels: readonly string[];
};

const screens: readonly Screen[] = [
  {
    id: "overview",
    title: "Overview",
    eyebrow: "Product health",
    summary: "API, workers, queues, payments and alerts in one read-only surface.",
    panels: ["API readiness", "Worker queues", "Payment webhook", "Active alerts"],
  },
  {
    id: "jobs",
    title: "Jobs",
    eyebrow: "Execution state",
    summary: "Search, filters and job detail will use safe backend DTOs only.",
    panels: ["Status filters", "Delivery state", "Reservation state", "Bounded errors"],
  },
  {
    id: "users",
    title: "Users",
    eyebrow: "Operator lookup",
    summary: "User views stay minimal by default and avoid raw PII-heavy output.",
    panels: ["Safe profile", "Balance snapshot", "Recent jobs", "Payment summary"],
  },
  {
    id: "payments",
    title: "Payments",
    eyebrow: "Ledger-backed billing",
    summary: "Payment state comes from protected billing endpoints, not redirects.",
    panels: ["Intent history", "Webhook inbox", "Reconciliation", "Refund state"],
  },
  {
    id: "providers",
    title: "Providers",
    eyebrow: "Model health",
    summary: "Provider visibility uses curated classes and bounded status labels.",
    panels: ["Circuit state", "Rate limits", "Fallback health", "Waste tracking"],
  },
  {
    id: "media",
    title: "Media Safety",
    eyebrow: "Upload and video policy",
    summary: "Media safety screens show policy decisions without raw URLs or storage keys.",
    panels: ["Upload rejects", "Probe policy", "Fast path", "Queue pressure"],
  },
  {
    id: "referrals",
    title: "Referrals",
    eyebrow: "Aggregate abuse signals",
    summary: "Referral views expose code-level aggregates without invited-user lists.",
    panels: ["Code stats", "Suspicious volume", "Activation ratio", "Future freeze flag"],
  },
  {
    id: "alerts",
    title: "Alerts",
    eyebrow: "Operational incidents",
    summary: "Alerts should explain impact and next check without sensitive payloads.",
    panels: ["Critical alerts", "Money risk", "Provider risk", "Security config"],
  },
  {
    id: "audit",
    title: "Audit Log",
    eyebrow: "Operator actions",
    summary: "Mutation actions will require reason, idempotency and audit records.",
    panels: ["Actor", "Action", "Target", "Result"],
  },
  {
    id: "config",
    title: "Config Health",
    eyebrow: "Non-secret flags",
    summary: "Config health must show only flags, readiness and policy classes.",
    panels: ["Admin auth", "Media flags", "Provider flags", "Webhook posture"],
  },
];

type ErrorBoundaryState = {
  hasError: boolean;
};

type OverviewState = {
  data?: OverviewDTO;
  error?: AdminApiError;
  loading: boolean;
};

class ErrorBoundary extends Component<{ children: ReactNode }, ErrorBoundaryState> {
  state: ErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError(_error: unknown): ErrorBoundaryState {
    return { hasError: true };
  }

  render(): ReactNode {
    if (this.state.hasError) {
      return (
        <main className="shell__content" role="alert">
          <section className="surface surface--alert">
            <p className="eyebrow">Safe fallback</p>
            <h1>Operator console error</h1>
            <p>The screen stopped without rendering internal details.</p>
          </section>
        </main>
      );
    }
    return this.props.children;
  }
}

function findScreen(id: ScreenId): Screen {
  return screens.find((screen) => screen.id === id) ?? screens[0];
}

function statusLabel(status: OverviewCardDTO["status"]): string {
  if (status === "ok") {
    return "OK";
  }
  if (status === "warning") {
    return "Warning";
  }
  if (status === "critical") {
    return "Critical";
  }
  return "Not wired";
}

function formatGeneratedAt(value?: string): string {
  if (!value) {
    return "not loaded";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "not loaded";
  }
  return date.toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

export function App() {
  const [activeScreen, setActiveScreen] = useState<ScreenId>("overview");
  const [tokenDraft, setTokenDraft] = useState("");
  const [adminToken, setAdminToken] = useState("");
  const [overview, setOverview] = useState<OverviewState>({ loading: false });
  const screen = findScreen(activeScreen);
  const sessionState = adminToken ? "Protected session" : "Token required";
  const tokenState = adminToken ? "in memory" : "not set";
  const adminClient = useMemo(() => createAdminClient({ tokenProvider: () => adminToken }), [adminToken]);
  const riskSummary = useMemo(
    () => [
      "read-only dashboard",
      "no direct provider calls",
      "no localStorage token",
      "no mutation actions",
    ],
    [],
  );

  useEffect(() => {
    if (activeScreen !== "overview") {
      return;
    }
    if (!adminToken) {
      setOverview({ loading: false });
      return;
    }
    const controller = new AbortController();
    setOverview((current) => ({ data: current.data, loading: true }));
    adminClient
      .request<OverviewDTO>("/admin/overview", { signal: controller.signal })
      .then((data) => setOverview({ data, loading: false }))
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          setOverview({ error: toSafeAdminError(error), loading: false });
        }
      });
    return () => controller.abort();
  }, [activeScreen, adminClient, adminToken]);

  function handleTokenSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setAdminToken(tokenDraft.trim());
    setTokenDraft("");
  }

  function clearSession() {
    setAdminToken("");
    setTokenDraft("");
    setOverview({ loading: false });
  }

  return (
    <ErrorBoundary>
      <div className="shell">
        <aside className="sidebar" aria-label="Operator sections">
          <div className="brand">
            <span className="brand__mark">N</span>
            <div>
              <strong>NeiroHub</strong>
              <span>Operator</span>
            </div>
          </div>

          <nav className="nav">
            {screens.map((item) => (
              <button
                className={item.id === activeScreen ? "nav__item nav__item--active" : "nav__item"}
                key={item.id}
                onClick={() => setActiveScreen(item.id)}
                type="button"
              >
                <span>{item.title}</span>
                <small>{item.eyebrow}</small>
              </button>
            ))}
          </nav>
        </aside>

        <main className="main">
          <header className="topbar">
            <div>
              <p className="eyebrow">Admin console</p>
              <h1>{screen.title}</h1>
            </div>
            <div className="session" aria-label="Admin session status">
              <span className={adminToken ? "dot dot--ok" : "dot"} />
              <div>
                <strong>{sessionState}</strong>
                <span>{tokenState}</span>
              </div>
            </div>
          </header>

          <section className="auth-panel surface" aria-labelledby="admin-token-title">
            <div>
              <p className="eyebrow">Admin token</p>
              <h2 id="admin-token-title">Session gate</h2>
            </div>
            <form className="token-form" onSubmit={handleTokenSubmit}>
              <input
                autoComplete="off"
                aria-label="Admin token"
                onChange={(event) => setTokenDraft(event.target.value)}
                placeholder="X-Admin-Token"
                type="password"
                value={tokenDraft}
              />
              <button type="submit">Use token</button>
              <button className="button-secondary" onClick={clearSession} type="button">
                Clear
              </button>
            </form>
          </section>

          <section className="screen-grid">
            <article className="surface surface--hero">
              <p className="eyebrow">{screen.eyebrow}</p>
              <h2>{screen.summary}</h2>
              <div className="risk-list" aria-label="Stage safety posture">
                {riskSummary.map((item) => (
                  <span key={item}>{item}</span>
                ))}
              </div>
            </article>

            {screen.id === "overview" ? (
              <OverviewPanel adminTokenSet={Boolean(adminToken)} overview={overview} />
            ) : (
              <div className="panel-grid">
                {screen.panels.map((panel) => (
                  <article className="surface panel" key={panel}>
                    <p className="eyebrow">Pending backend contract</p>
                    <h3>{panel}</h3>
                    <p>Read-only placeholder</p>
                  </article>
                ))}
              </div>
            )}
          </section>
        </main>
      </div>
    </ErrorBoundary>
  );
}

function OverviewPanel({ adminTokenSet, overview }: { adminTokenSet: boolean; overview: OverviewState }) {
  if (!adminTokenSet) {
    return (
      <article className="surface panel panel--wide" role="status">
        <p className="eyebrow">Auth required</p>
        <h3>Overview is locked</h3>
        <p>Enter an admin token to load the read-only operational summary.</p>
      </article>
    );
  }

  if (overview.loading && !overview.data) {
    return (
      <article className="surface panel panel--wide" role="status">
        <p className="eyebrow">Loading</p>
        <h3>Loading overview</h3>
        <p>Requesting safe bounded summaries from the admin API.</p>
      </article>
    );
  }

  if (overview.error) {
    return (
      <article className="surface panel panel--wide" role="alert">
        <p className="eyebrow">Safe error</p>
        <h3>{overview.error.message}</h3>
        <p>Code: {overview.error.code}</p>
      </article>
    );
  }

  if (!overview.data) {
    return (
      <article className="surface panel panel--wide" role="status">
        <p className="eyebrow">No data</p>
        <h3>Overview is not loaded</h3>
        <p>The admin API has not returned an overview yet.</p>
      </article>
    );
  }

  return (
    <div className="overview-stack">
      <div className="overview-meta" aria-live="polite">
        <span>Generated: {formatGeneratedAt(overview.data.generated_at)}</span>
        {overview.loading ? <span>Refreshing</span> : null}
      </div>
      <div className="overview-grid">
        {overview.data.cards.map((card) => (
          <article className={`surface panel overview-card overview-card--${card.status}`} key={card.id}>
            <div className="overview-card__header">
              <p className="eyebrow">{statusLabel(card.status)}</p>
              <span className={`status-pill status-pill--${card.status}`}>{statusLabel(card.status)}</span>
            </div>
            <h3>{card.title}</h3>
            <p>{card.summary}</p>
            {card.metrics && card.metrics.length > 0 ? (
              <dl className="metric-list">
                {card.metrics.map((metric) => (
                  <div key={`${card.id}:${metric.label}`}>
                    <dt>{metric.label}</dt>
                    <dd className={metric.status ? `metric-value metric-value--${metric.status}` : "metric-value"}>
                      {metric.value}
                    </dd>
                  </div>
                ))}
              </dl>
            ) : null}
          </article>
        ))}
      </div>
    </div>
  );
}
