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

const THEME_OPTIONS: Array<{ id: ThemeMode; label: string; text: string }> = [
  { id: "system", label: "Системная", text: "Как в VK или устройстве" },
  { id: "light", label: "Светлая", text: "Чистый рабочий режим" },
  { id: "dark", label: "Тёмная", text: "Мягкий контраст вечером" },
];

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
      <header className="screen-title">
        <span>Settings</span>
        <h1>Настройки</h1>
        <p>Тема, баланс и локальные данные Mini App в одном месте.</p>
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
                aria-label={`${option.label}: ${option.text}`}
                onClick={() => onThemeModeChange(option.id)}
              >
                <span>{option.label}</span>
                <small>{option.text}</small>
              </Button>
            );
          })}
        </div>
      </section>

      <section className="settings-section" aria-labelledby="settings-balance-title">
        <div className="section-head">
          <h2 id="settings-balance-title">Баланс</h2>
          <Button
            type="button"
            className="quiet-action"
            mode="secondary"
            appearance="neutral"
            size="m"
            onClick={onRefreshBalance}
          >
            Обновить
          </Button>
        </div>
        <div className="settings-balance" aria-live="polite">
          <span>Кредиты</span>
          <strong>{balance === null ? "..." : creditsLabel(balance)}</strong>
          <small>Источник истины — backend `/miniapp/balance`, не localStorage.</small>
        </div>
      </section>

      <section className="settings-section" aria-labelledby="settings-payments-title">
        <h2 id="settings-payments-title">История платежей</h2>
        <div className="settings-empty">
          Backend пока не отдаёт список платежей или ledger entries для Mini App.
          Раздел готов к подключению после отдельного BFF endpoint.
        </div>
      </section>

      <section className="settings-section" aria-labelledby="settings-history-title">
        <div className="section-head">
          <h2 id="settings-history-title">Сводная история</h2>
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
      </section>

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
