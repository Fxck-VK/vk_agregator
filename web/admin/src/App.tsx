import { Component, FormEvent, ReactNode, useMemo, useState } from "react";

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
            <p>Экран остановлен без вывода внутренних деталей.</p>
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

export function App() {
  const [activeScreen, setActiveScreen] = useState<ScreenId>("overview");
  const [tokenDraft, setTokenDraft] = useState("");
  const [adminToken, setAdminToken] = useState("");
  const screen = findScreen(activeScreen);
  const sessionState = adminToken ? "Protected session" : "Token required";
  const tokenState = adminToken ? "in memory" : "not set";
  const riskSummary = useMemo(
    () => [
      "read-only skeleton",
      "no direct provider calls",
      "no localStorage token",
      "no mutation actions",
    ],
    [],
  );

  function handleTokenSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setAdminToken(tokenDraft.trim());
    setTokenDraft("");
  }

  function clearSession() {
    setAdminToken("");
    setTokenDraft("");
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

            <div className="panel-grid">
              {screen.panels.map((panel) => (
                <article className="surface panel" key={panel}>
                  <p className="eyebrow">Pending backend contract</p>
                  <h3>{panel}</h3>
                  <p>Read-only placeholder</p>
                </article>
              ))}
            </div>
          </section>
        </main>
      </div>
    </ErrorBoundary>
  );
}
