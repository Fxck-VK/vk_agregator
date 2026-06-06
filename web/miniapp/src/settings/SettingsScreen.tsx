import { useMemo, useState } from "react";
import { Button, NativeSelect } from "@vkontakte/vkui";
import { statusKind, statusLabel, type Job } from "../api/client";
import { MODALITIES, modalityByOperation, type ModalityId } from "../chat/types";
import type { ThemeMode } from "./theme";

type SettingsScreenProps = {
  themeMode: ThemeMode;
  balance: number | null;
  jobs: Job[];
  loading: boolean;
  onThemeModeChange: (mode: ThemeMode) => void;
  onClearLocalHistory: () => void;
  onRefreshBalance: () => void;
};

type HistoryFilter = "all" | ModalityId;
type PaymentHistoryItem = {
  id: string;
  title: string;
  amount: number;
  created_at: string;
  status: string;
};

const THEME_OPTIONS: Array<{ id: ThemeMode; label: string }> = [
  { id: "system", label: "Система" },
  { id: "light", label: "Светлая" },
  { id: "dark", label: "Тёмная" },
];

const PAYMENT_HISTORY: PaymentHistoryItem[] = [];

function dateLabel(value: string): string {
  const ts = Date.parse(value);
  if (!Number.isFinite(ts)) return "";
  return new Intl.DateTimeFormat("ru-RU", {
    day: "2-digit",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  }).format(ts);
}

function creditsLabel(value: number): string {
  return `${value.toLocaleString("ru-RU")} кр.`;
}

function historyLabel(job: Job): string {
  const kind = statusKind(job.status);
  if (kind === "done") return "Готово";
  if (kind === "failed") return "Ошибка";
  return statusLabel(job.status);
}

export function SettingsScreen({
  themeMode,
  balance,
  jobs,
  loading,
  onThemeModeChange,
  onClearLocalHistory,
  onRefreshBalance,
}: SettingsScreenProps) {
  const [historyFilter, setHistoryFilter] = useState<HistoryFilter>("all");
  const [topUpNotice, setTopUpNotice] = useState("");
  const sortedJobs = useMemo(
    () => [...jobs].sort((a, b) => b.created_at.localeCompare(a.created_at)),
    [jobs],
  );
  const visibleJobs = sortedJobs.filter((job) => {
    if (historyFilter === "all") return true;
    return modalityByOperation(job.operation).id === historyFilter;
  });

  return (
    <main className="settings-screen">
      <header className="screen-title settings-title">
        <h1>Настройки</h1>
      </header>

      <section className="settings-section" aria-labelledby="settings-theme-title">
        <div className="section-head">
          <h2 id="settings-theme-title">Тема</h2>
        </div>
        <div className="theme-options" role="group" aria-label="Тема приложения">
          {THEME_OPTIONS.map((option) => {
            const active = option.id === themeMode;
            return (
              <Button
                key={option.id}
                type="button"
                className={"theme-option" + (active ? " is-active" : "")}
                mode={active ? "primary" : "secondary"}
                appearance={active ? "accent" : "neutral"}
                size="l"
                aria-label={option.label}
                onClick={() => onThemeModeChange(option.id)}
              >
                <span>{option.label}</span>
              </Button>
            );
          })}
        </div>
      </section>

      <section className="settings-section" aria-labelledby="settings-balance-title">
        <h2 id="settings-balance-title">Баланс</h2>
        <div className="settings-balance-card" aria-live="polite">
          <div className="settings-balance-card__main">
            <span>Доступно</span>
            <strong>{balance === null ? "..." : creditsLabel(balance)}</strong>
          </div>
          <div className="settings-balance-card__actions">
            <Button
              type="button"
              mode="secondary"
              appearance="neutral"
              size="m"
              onClick={onRefreshBalance}
            >
              Обновить
            </Button>
            <Button
              type="button"
              mode="primary"
              appearance="accent"
              size="m"
              onClick={() => {
                setTopUpNotice("Пополнение скоро появится. Баланс обновится после подтверждения платежа.");
              }}
            >
              Пополнить
            </Button>
          </div>
          {topUpNotice && <p className="settings-notice">{topUpNotice}</p>}
        </div>
      </section>

      <details className="settings-accordion">
        <summary>
          <span>История платежей</span>
          <small>{PAYMENT_HISTORY.length}</small>
        </summary>
        {PAYMENT_HISTORY.length === 0 ? (
          <div className="settings-empty">Платежей пока нет.</div>
        ) : (
          <div className="settings-history-list">
            {PAYMENT_HISTORY.map((item) => (
              <article key={item.id} className="settings-history-row">
                <div>
                  <span>{item.title}</span>
                  <strong>{creditsLabel(item.amount)}</strong>
                </div>
                <div className="settings-history-row__meta">
                  <time>{dateLabel(item.created_at)}</time>
                  <span className="history-status">{item.status}</span>
                </div>
              </article>
            ))}
          </div>
        )}
      </details>

      <details className="settings-accordion" open>
        <summary>
          <span>Сводная история</span>
          <small>{visibleJobs.length}</small>
        </summary>
        <div className="settings-accordion__tools">
          <NativeSelect
            className="settings-history-filter"
            value={historyFilter}
            aria-label="Фильтр типа генерации"
            onChange={(event) => setHistoryFilter(event.target.value as HistoryFilter)}
          >
            <option value="all">Все типы</option>
            {MODALITIES.map((item) => (
              <option key={item.id} value={item.id}>
                {item.label}
              </option>
            ))}
          </NativeSelect>
        </div>
        {loading ? (
          <div className="settings-empty">Загружаем историю генераций</div>
        ) : visibleJobs.length === 0 ? (
          <div className="settings-empty">Нет генераций под выбранный фильтр.</div>
        ) : (
          <div className="settings-history-list">
            {visibleJobs.slice(0, 30).map((job) => {
              const modality = modalityByOperation(job.operation);
              const kind = statusKind(job.status);
              const cost = job.cost_captured > 0 ? job.cost_captured : job.cost_estimate;
              return (
                <article key={job.id} className="settings-history-row">
                  <div>
                    <span>{modality.label}</span>
                    <strong>{historyLabel(job)}</strong>
                  </div>
                  <div className="settings-history-row__meta">
                    <time>{dateLabel(job.created_at)}</time>
                    {cost > 0 && <small>{creditsLabel(cost)}</small>}
                    <span className={"history-status history-status--" + kind}>{kind}</span>
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </details>

      <section className="settings-section" aria-labelledby="settings-privacy-title">
        <h2 id="settings-privacy-title">Локальные данные</h2>
        <div className="privacy-card">
          <p>
            На устройстве хранятся только UI-настройки: активная вкладка, тема
            и метаданные диалогов `id` / `title` / `last_activity_at`.
          </p>
          <p>
            Prompt, ответы, launch params, токены, баланс, provider details и
            artifact URLs не сохраняются в localStorage.
          </p>
          <Button
            type="button"
            mode="secondary"
            appearance="neutral"
            size="l"
            stretched
            onClick={onClearLocalHistory}
          >
            Очистить локальную историю
          </Button>
        </div>
      </section>
    </main>
  );
}
