import { useMemo, useState } from "react";
import { Button } from "@vkontakte/vkui";
import { statusKind, statusLabel, type Job } from "../api/client";
import { modalityByOperation, type ModalityId } from "../chat/types";
import neuroHubAvatar from "../assets/neurohub-avatar.png";
import { formatCredits } from "../ui/credits";
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

type HistoryFilter = "all" | ModalityId | "chat";

const THEME_OPTIONS: Array<{ id: ThemeMode; label: string }> = [
  { id: "system", label: "Авто" },
  { id: "dark", label: "Тёмная" },
  { id: "light", label: "Светлая" },
];

const FILTER_OPTIONS: Array<{ id: HistoryFilter; label: string }> = [
  { id: "all", label: "Все" },
  { id: "image", label: "Картинки" },
  { id: "video", label: "Видео" },
  { id: "text", label: "Диалоги" },
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

function historyLabel(job: Job): string {
  const kind = statusKind(job.status);
  if (kind === "done") return "Готово";
  if (kind === "failed") return "Ошибка";
  return statusLabel(job.status);
}

function typeColor(operation: string): string {
  const id = modalityByOperation(operation).id;
  if (id === "image") return "#a855f7";
  if (id === "video") return "#ec4899";
  return "#22d3ee";
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
  const [spinning, setSpinning] = useState(false);
  const sortedJobs = useMemo(
    () => [...jobs].sort((a, b) => b.created_at.localeCompare(a.created_at)),
    [jobs],
  );
  const visibleJobs = sortedJobs.filter((job) => {
    if (historyFilter === "all") return true;
    if (historyFilter === "text") return job.operation === "text_generate";
    return modalityByOperation(job.operation).id === historyFilter;
  });

  const handleRefresh = () => {
    if (spinning) return;
    setSpinning(true);
    onRefreshBalance();
    window.setTimeout(() => setSpinning(false), 900);
  };

  return (
    <main className="settings-screen nh-scroll">
      <div className="nh-hero nh-hero--profile" aria-hidden="true">
        <img className="nh-hero__img" src={neuroHubAvatar} alt="" />
        <div className="nh-hero__fade" />
        <div className="nh-hero__avatar">
          <img src={neuroHubAvatar} alt="" />
        </div>
      </div>

      <div className="settings-hero-copy">
        <h1 className="nh-page-title">НейроХаб</h1>
        <p className="nh-page-sub">инструменты для нового поколения</p>
      </div>

      <section className="settings-card" aria-labelledby="settings-theme-title">
        <h2 id="settings-theme-title" style={{ margin: "0 0 14px", fontSize: "15px" }}>
          Тема оформления
        </h2>
        <div className="theme-segment" role="group" aria-label="Тема приложения">
          {THEME_OPTIONS.map((option) => (
            <button
              key={option.id}
              type="button"
              className={"theme-segment__btn" + (themeMode === option.id ? " is-active" : "")}
              onClick={() => onThemeModeChange(option.id)}
            >
              <span>{option.label}</span>
            </button>
          ))}
        </div>
      </section>

      <section className="settings-card" aria-labelledby="settings-balance-title">
        <h2 id="settings-balance-title" style={{ margin: "0 0 14px", fontSize: "15px" }}>
          Баланс
        </h2>
        <div className="settings-balance-hero">
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <div>
              <p style={{ margin: "0 0 4px", fontSize: "12px", color: "var(--fg-muted)" }}>
                Текущий баланс
              </p>
              <strong style={{ fontSize: "30px", fontWeight: 800, background: "var(--gradient-brand)", WebkitBackgroundClip: "text", backgroundClip: "text", WebkitTextFillColor: "transparent" }}>
                {balance === null ? "..." : formatCredits(balance)}
              </strong>
            </div>
            <button
              type="button"
              className="chat__history-btn"
              aria-label="Обновить баланс"
              onClick={handleRefresh}
            >
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                className={spinning ? "nh-spin" : ""}
                aria-hidden="true"
              >
                <path d="M3 12a9 9 0 1 0 3-6.7M3 3v6h6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </button>
          </div>
        </div>
        <Button
          type="button"
          mode="primary"
          appearance="accent"
          size="l"
          stretched
          onClick={() => {
            setTopUpNotice("Пополнение скоро появится. Баланс обновится после подтверждения платежа.");
          }}
        >
          Пополнить баланс
        </Button>
        {topUpNotice && <p className="settings-notice">{topUpNotice}</p>}
      </section>

      <section className="settings-card" aria-labelledby="settings-history-title">
        <h2 id="settings-history-title" style={{ margin: "0 0 14px", fontSize: "15px" }}>
          История запросов
        </h2>
        <div className="settings-filter-pills" role="group" aria-label="Фильтр истории">
          {FILTER_OPTIONS.map((option) => (
            <button
              key={option.id}
              type="button"
              className={"settings-filter-pill" + (historyFilter === option.id ? " is-active" : "")}
              onClick={() => setHistoryFilter(option.id)}
            >
              {option.label}
            </button>
          ))}
        </div>
        {loading ? (
          <div className="settings-empty">Загружаем историю генераций</div>
        ) : visibleJobs.length === 0 ? (
          <div className="settings-empty">Нет записей</div>
        ) : (
          <div className="settings-history-list" style={{ padding: 0 }}>
            {visibleJobs.slice(0, 30).map((job) => {
              const modality = modalityByOperation(job.operation);
              const color = typeColor(job.operation);
              const cost = job.cost_captured > 0 ? job.cost_captured : job.cost_estimate;
              return (
                <article key={job.id} className="settings-history-row">
                  <div
                    style={{
                      width: 36,
                      height: 36,
                      borderRadius: 10,
                      background: `${color}18`,
                      border: `1px solid ${color}35`,
                      display: "grid",
                      placeItems: "center",
                      color,
                      flexShrink: 0,
                    }}
                    aria-hidden="true"
                  >
                    <span style={{ fontSize: 11, fontWeight: 800 }}>{modality.label[0]}</span>
                  </div>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: 14, fontWeight: 600 }}>
                      {job.prompt?.slice(0, 48) || modality.label}
                    </div>
                    <div style={{ fontSize: 12, color: "var(--fg-muted)" }}>
                      {modality.label} · {historyLabel(job)} · {dateLabel(job.created_at)}
                      {cost > 0 ? ` · ${formatCredits(cost)}` : ""}
                    </div>
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </section>

      <section className="settings-card" aria-labelledby="settings-privacy-title">
        <h2 id="settings-privacy-title" style={{ margin: "0 0 12px", fontSize: "15px" }}>
          Локальные данные
        </h2>
        <p style={{ margin: "0 0 12px", color: "var(--fg-muted)", fontSize: 14, lineHeight: 1.45 }}>
          На устройстве хранятся только UI-настройки: активная вкладка, тема и метаданные диалогов.
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
      </section>
    </main>
  );
}
