import { Component, FormEvent, ReactNode, useEffect, useMemo, useState } from "react";
import { AdminApiError, createAdminClient, toSafeAdminError } from "./api/adminClient";
import type { OverviewCardDTO, OverviewDTO } from "./api/overview";
import { JobsScreen } from "./screens/JobsScreen";
import { PaymentsScreen } from "./screens/PaymentsScreen";
import { ConfigHealthScreen, MediaSafetyScreen, ProvidersScreen } from "./screens/ProviderMediaScreens";
import { AuditLogScreen, ReferralsScreen, UsersScreen } from "./screens/UsersReferralsAuditScreens";

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
    title: "Обзор",
    eyebrow: "Здоровье продукта",
    summary: "API, воркеры, очереди, платежи и алерты в одной безопасной панели.",
    panels: ["Готовность API", "Очереди воркеров", "Платежные вебхуки", "Активные алерты"],
  },
  {
    id: "jobs",
    title: "Задачи",
    eyebrow: "Выполнение",
    summary: "Поиск, фильтры, статусы, очереди, доставки и безопасные детали job.",
    panels: ["Фильтры статуса", "Состояние доставки", "Резервы баланса", "Классы ошибок"],
  },
  {
    id: "users",
    title: "Пользователи",
    eyebrow: "Поиск оператора",
    summary: "Безопасные сводки пользователей без raw PII и лишних идентификаторов.",
    panels: ["Безопасный профиль", "Баланс", "Последние задачи", "Сводка платежей"],
  },
  {
    id: "payments",
    title: "Платежи",
    eyebrow: "Ledger-биллинг",
    summary: "Платежи, ledger, reconciliation и возвраты только из защищенного backend.",
    panels: ["История intents", "Webhook inbox", "Reconciliation", "Возвраты"],
  },
  {
    id: "providers",
    title: "Провайдеры",
    eyebrow: "Модели",
    summary: "Здоровье provider/model классов, circuit breaker, fallback и потери денег.",
    panels: ["Circuit state", "Rate limits", "Fallback", "Provider waste"],
  },
  {
    id: "media",
    title: "Медиа-безопасность",
    eyebrow: "Политики медиа",
    summary: "Политики загрузок, видео fast path, очереди и media risks без private URLs.",
    panels: ["Отклонения upload", "Probe policy", "Fast path", "Давление очередей"],
  },
  {
    id: "referrals",
    title: "Рефералы",
    eyebrow: "Агрегаты и abuse",
    summary: "Статистика referral-кодов и подозрительная активность без списков приглашенных.",
    panels: ["Статистика кодов", "Подозрительный объем", "Активация", "Freeze flag"],
  },
  {
    id: "alerts",
    title: "Алерты",
    eyebrow: "Инциденты",
    summary: "Операционные предупреждения: что случилось, где смотреть и что чинить.",
    panels: ["Критичные алерты", "Риск денег", "Риск провайдеров", "Security config"],
  },
  {
    id: "audit",
    title: "Аудит",
    eyebrow: "Действия оператора",
    summary: "Журнал операторских действий, причин, результатов и correlation/request id.",
    panels: ["Кто сделал", "Действие", "Цель", "Результат"],
  },
  {
    id: "config",
    title: "Конфиг",
    eyebrow: "Несекретные флаги",
    summary: "Только безопасные runtime-флаги, readiness и policy-классы без секретов.",
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
            <p className="eyebrow">Безопасный fallback</p>
            <h1>Ошибка админки</h1>
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

function statusLabel(status: OverviewCardDTO["status"]): string {
  if (status === "ok") {
    return "OK";
  }
  if (status === "warning") {
    return "Внимание";
  }
  if (status === "critical") {
    return "Критично";
  }
  return "Не подключено";
}

function formatGeneratedAt(value?: string): string {
  if (!value) {
    return "не загружено";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "не загружено";
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
  const [loginError, setLoginError] = useState("");
  const [overview, setOverview] = useState<OverviewState>({ loading: false });
  const screen = findScreen(activeScreen);
  const sessionState = adminToken ? "Доступ открыт" : "Нужен токен";
  const tokenState = adminToken ? "только в памяти вкладки" : "не введен";
  const adminClient = useMemo(() => createAdminClient({ tokenProvider: () => adminToken }), [adminToken]);
  const riskSummary = useMemo(
    () => [
      "внутренняя панель",
      "токен только в памяти",
      "без прямого доступа к БД",
      "provider/billing только через backend",
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
          const safeError = toSafeAdminError(error);
          if (safeError.code === "admin_auth_required" || safeError.code === "admin_forbidden") {
            setAdminToken("");
            setTokenDraft("");
            setLoginError("Токен не принят. Проверьте локальный ADMIN_TOKEN и войдите снова.");
            setOverview({ loading: false });
            return;
          }
          setOverview({ error: safeError, loading: false });
        }
      });
    return () => controller.abort();
  }, [activeScreen, adminClient, adminToken]);

  function handleTokenSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const nextToken = tokenDraft.trim();
    if (!nextToken) {
      setLoginError("Введите ADMIN_TOKEN из локального .env.");
      return;
    }
    setLoginError("");
    setAdminToken(nextToken);
    setTokenDraft("");
  }

  function clearSession() {
    setAdminToken("");
    setTokenDraft("");
    setLoginError("");
    setOverview({ loading: false });
  }

  if (!adminToken) {
    return (
      <ErrorBoundary>
        <LoginScreen
          loginError={loginError}
          onTokenChange={setTokenDraft}
          onSubmit={handleTokenSubmit}
          tokenDraft={tokenDraft}
        />
      </ErrorBoundary>
    );
  }

  return (
    <ErrorBoundary>
      <div className="shell">
        <aside className="sidebar" aria-label="Разделы админки">
          <div className="brand">
            <span className="brand__mark">N</span>
            <div>
              <strong>NeiroHub</strong>
              <span>Операторская</span>
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
              <p className="eyebrow">Админка</p>
              <h1>{screen.title}</h1>
            </div>
            <div className="session" aria-label="Состояние админской сессии">
              <span className={adminToken ? "dot dot--ok" : "dot"} />
              <div>
                <strong>{sessionState}</strong>
                <span>{tokenState}</span>
              </div>
              <button className="session__logout" onClick={clearSession} type="button">
                Выйти
              </button>
            </div>
          </header>

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
            ) : screen.id === "jobs" ? (
              <JobsScreen adminTokenSet={Boolean(adminToken)} client={adminClient} />
            ) : screen.id === "payments" ? (
              <PaymentsScreen adminTokenSet={Boolean(adminToken)} client={adminClient} />
            ) : screen.id === "users" ? (
              <UsersScreen adminTokenSet={Boolean(adminToken)} client={adminClient} />
            ) : screen.id === "providers" ? (
              <ProvidersScreen adminTokenSet={Boolean(adminToken)} client={adminClient} />
            ) : screen.id === "media" ? (
              <MediaSafetyScreen adminTokenSet={Boolean(adminToken)} client={adminClient} />
            ) : screen.id === "referrals" ? (
              <ReferralsScreen adminTokenSet={Boolean(adminToken)} client={adminClient} />
            ) : screen.id === "audit" ? (
              <AuditLogScreen adminTokenSet={Boolean(adminToken)} client={adminClient} />
            ) : screen.id === "config" ? (
              <ConfigHealthScreen adminTokenSet={Boolean(adminToken)} client={adminClient} />
            ) : (
              <div className="panel-grid">
                {screen.panels.map((panel) => (
                  <article className="surface panel" key={panel}>
                    <p className="eyebrow">Backend contract не подключен</p>
                    <h3>{panel}</h3>
                    <p>Read-only placeholder. Здесь появятся безопасные backend DTO.</p>
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

function LoginScreen({
  loginError,
  onSubmit,
  onTokenChange,
  tokenDraft,
}: {
  loginError: string;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onTokenChange: (value: string) => void;
  tokenDraft: string;
}) {
  return (
    <main className="login-screen">
      <section className="surface login-card" aria-labelledby="admin-token-title">
        <div className="login-card__heading">
          <span className="brand__mark">N</span>
          <div>
            <p className="eyebrow">Локальная админка</p>
            <h1 id="admin-token-title">Вход в админку</h1>
          </div>
        </div>
        <p className="login-card__copy">
          Введите локальный `ADMIN_TOKEN`. Токен хранится только в памяти вкладки, не пишется в localStorage и не
          появляется в логах или ошибках.
        </p>
        <form className="token-form token-form--login" onSubmit={onSubmit}>
          <label>
            <span>Админский токен</span>
            <input
              autoComplete="off"
              aria-label="Админский токен"
              onChange={(event) => onTokenChange(event.target.value)}
              placeholder="ADMIN_TOKEN"
              type="password"
              value={tokenDraft}
            />
          </label>
          <button type="submit">Войти</button>
        </form>
        {loginError ? (
          <p className="login-error" role="alert">
            {loginError}
          </p>
        ) : null}
        <div className="login-hints" aria-label="Что внутри админки">
          <span>Обзор продукта</span>
          <span>Задачи и очереди</span>
          <span>Платежи и ledger</span>
          <span>Провайдеры и media safety</span>
        </div>
      </section>
    </main>
  );
}

function OverviewPanel({ adminTokenSet, overview }: { adminTokenSet: boolean; overview: OverviewState }) {
  if (!adminTokenSet) {
    return (
      <article className="surface panel panel--wide" role="status">
        <p className="eyebrow">Нужен доступ</p>
        <h3>Обзор закрыт</h3>
        <p>Введите админский токен, чтобы загрузить операционную сводку.</p>
      </article>
    );
  }

  if (overview.loading && !overview.data) {
    return (
      <article className="surface panel panel--wide" role="status">
        <p className="eyebrow">Loading</p>
        <h3>Загружаю обзор</h3>
        <p>Запрашиваю безопасную ограниченную сводку из admin API.</p>
      </article>
    );
  }

  if (overview.error) {
    return (
      <article className="surface panel panel--wide" role="alert">
        <p className="eyebrow">Безопасная ошибка</p>
        <h3>{overview.error.message}</h3>
        <p>Код: {overview.error.code}</p>
      </article>
    );
  }

  if (!overview.data) {
    return (
      <article className="surface panel panel--wide" role="status">
        <p className="eyebrow">Нет данных</p>
        <h3>Обзор еще не загружен</h3>
        <p>Admin API пока не вернул данные.</p>
      </article>
    );
  }

  return (
    <div className="overview-stack">
      <div className="overview-meta" aria-live="polite">
        <span>Сгенерировано: {formatGeneratedAt(overview.data.generated_at)}</span>
        {overview.loading ? <span>Обновляется</span> : null}
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
