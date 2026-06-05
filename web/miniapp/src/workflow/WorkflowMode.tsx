import { useEffect, useMemo, useState } from "react";
import { Button, NativeSelect, Textarea } from "@vkontakte/vkui";
import {
  apiUserMessage,
  estimateJob,
  isTerminal,
  statusKind,
  statusLabel,
  type EstimateResponse,
  type Job,
} from "../api/client";
import { ResultCard } from "../components/ResultCard";
import { type Chat, type ChatMessage, MODALITIES, modalityById, modalityByOperation, type ModalityId } from "../chat/types";
import type { VkUser } from "../hooks/useBridge";

type WorkflowScreen = "home" | "generate" | "status" | "result" | "history";

type WorkflowModeProps = {
  user: VkUser;
  balance: number | null;
  jobs: Job[];
  chats: Chat[];
  loading: boolean;
  submitting: boolean;
  onCreateJob: (prompt: string, request: { operation: string; modelId: string }) => Promise<Job | null>;
  onClearLocalHistory: () => void;
};

const ESTIMATE_DEBOUNCE_MS = 450;
const PROMPT_LIMIT = 2000;

const QUICK_SCENARIOS: Array<{
  title: string;
  text: string;
  modality: ModalityId;
  prompt: string;
}> = [
  {
    title: "Пост для сообщества",
    text: "Короткий текст с понятным CTA",
    modality: "text",
    prompt: "Сделай VK-пост для сообщества: новость, польза, короткий призыв к действию.",
  },
  {
    title: "Визуал к анонсу",
    text: "Изображение для ленты",
    modality: "image",
    prompt: "Создай промпт для яркого, но минималистичного изображения к VK-посту.",
  },
  {
    title: "Короткое видео",
    text: "Сценарий для клипа",
    modality: "video",
    prompt: "Опиши короткое вертикальное видео для VK: 5-7 секунд, один главный объект, чистый фон.",
  },
];

const HISTORY_STATUS_FILTERS = [
  { id: "all", label: "Все" },
  { id: "progress", label: "В работе" },
  { id: "done", label: "Готово" },
  { id: "failed", label: "Ошибка" },
] as const;

const TIMELINE = [
  {
    id: "checking",
    title: "Проверка",
    text: "Запрос принят и проходит базовую валидацию",
    statuses: new Set(["received", "validated"]),
  },
  {
    id: "reserving",
    title: "Резерв",
    text: "Бэкенд проверяет баланс и резервирует кредиты",
    statuses: new Set(["credits_reserved", "awaiting_payment"]),
  },
  {
    id: "queued",
    title: "Очередь",
    text: "Задача ждёт свободного воркера",
    statuses: new Set(["queued", "dispatching_provider"]),
  },
  {
    id: "generating",
    title: "Генерация",
    text: "Модель создаёт контент через backend job flow",
    statuses: new Set(["provider_submitted", "provider_pending", "provider_processing"]),
  },
  {
    id: "verifying",
    title: "Проверка результата",
    text: "Артефакт сохраняется и проходит безопасную обработку",
    statuses: new Set(["provider_succeeded", "postprocessing", "result_ready", "delivering"]),
  },
  {
    id: "done",
    title: "Готово",
    text: "Результат можно посмотреть и скопировать",
    statuses: new Set(["succeeded"]),
  },
];

function operationLabel(operation: string): string {
  return modalityByOperation(operation).label;
}

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

function sortedJobs(jobs: Job[]): Job[] {
  return [...jobs].sort((a, b) => b.created_at.localeCompare(a.created_at));
}

function messageForJob(job: Job | undefined, chats: Chat[]): ChatMessage | null {
  if (!job) return null;
  for (const chat of chats) {
    const msg = chat.messages.find((item) => item.jobId === job.id && item.role === "bot");
    if (msg) return msg;
  }
  return {
    id: "workflow-" + job.id,
    role: "bot",
    jobId: job.id,
    operation: job.operation,
    status: job.status,
    pending: !isTerminal(job.status),
    artifactIds: isTerminal(job.status) && statusKind(job.status) === "done" ? job.output_artifact_ids : undefined,
    createdAt: job.created_at,
  };
}

function timelineIndex(status: string): number {
  if (statusKind(status) === "failed") return TIMELINE.length - 1;
  const index = TIMELINE.findIndex((stage) => stage.statuses.has(status));
  return index === -1 ? Math.max(0, TIMELINE.length - 2) : index;
}

function ModeTabs({ screen, onScreen }: { screen: WorkflowScreen; onScreen: (screen: WorkflowScreen) => void }) {
  const tabs: Array<{ id: WorkflowScreen; label: string }> = [
    { id: "home", label: "Home" },
    { id: "generate", label: "Generate" },
    { id: "history", label: "History" },
  ];
  return (
    <nav className="workflow-tabs" aria-label="Workflow sections">
      {tabs.map((tab) => (
        <Button
          key={tab.id}
          type="button"
          className={"workflow-tabs__btn" + (screen === tab.id ? " is-active" : "")}
          mode={screen === tab.id ? "primary" : "tertiary"}
          appearance={screen === tab.id ? "neutral" : "neutral"}
          size="m"
          onClick={() => onScreen(tab.id)}
        >
          {tab.label}
        </Button>
      ))}
    </nav>
  );
}

export function WorkflowMode({
  user,
  balance,
  jobs,
  chats,
  loading,
  submitting,
  onCreateJob,
  onClearLocalHistory,
}: WorkflowModeProps) {
  const [screen, setScreen] = useState<WorkflowScreen>("home");
  const [modalityId, setModalityId] = useState<ModalityId>("text");
  const [modelId, setModelId] = useState(modalityById("text").models[0].id);
  const [prompt, setPrompt] = useState("");
  const [estimate, setEstimate] = useState<EstimateResponse | null>(null);
  const [estimateLoading, setEstimateLoading] = useState(false);
  const [estimateError, setEstimateError] = useState<string | null>(null);
  const [activeJobId, setActiveJobId] = useState<string | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [modalityFilter, setModalityFilter] = useState<"all" | ModalityId>("all");
  const [statusFilter, setStatusFilter] = useState<(typeof HISTORY_STATUS_FILTERS)[number]["id"]>("all");

  const recentJobs = useMemo(() => sortedJobs(jobs), [jobs]);
  const activeJob = activeJobId ? jobs.find((job) => job.id === activeJobId) : undefined;
  const activeMessage = messageForJob(activeJob, chats);
  const activePrompt = prompt.trim();
  const currentModality = modalityById(modalityId);
  const modelSelected = currentModality.models.some((model) => model.id === modelId);
  const trimmedPrompt = prompt.trim();
  const promptTooLong = prompt.length > PROMPT_LIMIT;
  const canSubmit =
    !!trimmedPrompt &&
    !promptTooLong &&
    modelSelected &&
    estimate?.enough_credits === true &&
    !submitting;

  useEffect(() => {
    const value = prompt.trim();
    setEstimate(null);
    setEstimateError(null);
    if (!value || promptTooLong || !modelSelected) {
      setEstimateLoading(false);
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(() => {
      setEstimateLoading(true);
      estimateJob({ operation: currentModality.operation, prompt: value, model_id: modelId })
        .then((data) => {
          if (cancelled) return;
          setEstimate(data);
        })
        .catch((error) => {
          if (cancelled) return;
          setEstimateError(apiUserMessage(error));
        })
        .finally(() => {
          if (!cancelled) setEstimateLoading(false);
        });
    }, ESTIMATE_DEBOUNCE_MS);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [currentModality.operation, modelId, modelSelected, prompt, promptTooLong]);

  useEffect(() => {
    if (activeJob && isTerminal(activeJob.status) && statusKind(activeJob.status) === "done" && screen === "status") {
      setScreen("result");
    }
  }, [activeJob, screen]);

  function changeModality(id: ModalityId) {
    const next = modalityById(id);
    setModalityId(id);
    setModelId(next.models[0]?.id ?? "");
  }

  function openScenario(modality: ModalityId, nextPrompt?: string) {
    changeModality(modality);
    if (nextPrompt) setPrompt(nextPrompt);
    setScreen("generate");
  }

  async function submitWorkflow() {
    if (!canSubmit) return;
    setSubmitError(null);
    const job = await onCreateJob(trimmedPrompt, {
      operation: currentModality.operation,
      modelId,
    });
    if (!job) {
      setSubmitError("Не удалось запустить генерацию");
      return;
    }
    setActiveJobId(job.id);
    setScreen("status");
  }

  function repeatJob(job: Job) {
    const modality = modalityByOperation(job.operation);
    changeModality(modality.id);
    setPrompt(job.prompt ?? "");
    setScreen("generate");
  }

  const filteredJobs = recentJobs.filter((job) => {
    const kind = statusKind(job.status);
    const modality = modalityByOperation(job.operation).id;
    return (modalityFilter === "all" || modalityFilter === modality) && (statusFilter === "all" || statusFilter === kind);
  });

  return (
    <main className="workflow">
      <ModeTabs screen={screen} onScreen={setScreen} />

      {screen === "home" && (
        <section className="workflow-screen">
          <div className="workflow-hero">
            <span className="workflow-kicker">Content workflow</span>
            <h1>{user.firstName}, создайте VK-пост без лишнего шума</h1>
            <p>Сначала выберите сценарий, затем проверьте стоимость и дождитесь результата в спокойном статус-экране.</p>
            <Button
              type="button"
              className="workflow-primary"
              mode="primary"
              size="l"
              onClick={() => setScreen("generate")}
            >
              Создать VK-пост
            </Button>
          </div>

          <div className="workflow-balance" aria-live="polite">
            <span>Баланс</span>
            <strong>{balance === null ? "..." : `${balance.toLocaleString("ru-RU")} кр.`}</strong>
            {balance !== null && balance <= 0 && <em>Нужны кредиты для запуска</em>}
          </div>

          <section className="workflow-section" aria-labelledby="quick-title">
            <h2 id="quick-title">Быстрые сценарии</h2>
            <div className="scenario-grid">
              {QUICK_SCENARIOS.map((scenario) => (
                <button
                  key={scenario.title}
                  type="button"
                  className="scenario-card"
                  onClick={() => openScenario(scenario.modality, scenario.prompt)}
                >
                  <span>{scenario.title}</span>
                  <small>{scenario.text}</small>
                </button>
              ))}
            </div>
          </section>

          <section className="workflow-section" aria-labelledby="recent-title">
            <div className="section-head">
              <h2 id="recent-title">Последние генерации</h2>
              <Button
                type="button"
                className="quiet-action"
                mode="secondary"
                appearance="neutral"
                size="m"
                onClick={() => setScreen("history")}
              >
                История
              </Button>
            </div>
            {loading ? (
              <div className="workflow-empty">Загружаем историю</div>
            ) : recentJobs.length === 0 ? (
              <div className="workflow-empty">Пока тихо. Первый пост начнётся здесь.</div>
            ) : (
              <JobList jobs={recentJobs.slice(0, 3)} onOpen={(job) => {
                setActiveJobId(job.id);
                setScreen(isTerminal(job.status) && statusKind(job.status) === "done" ? "result" : "status");
              }} />
            )}
          </section>
        </section>
      )}

      {screen === "generate" && (
        <section className="workflow-screen">
          <ScreenTitle eyebrow="Generate" title="Опишите будущий пост" text="Стоимость и доступность модели считает backend до запуска." />
          <div className="workflow-form">
            <div className="workflow-field">
              <label>Тип результата</label>
              <div className="workflow-segment" role="tablist">
                {MODALITIES.map((item) => (
                  <Button
                    key={item.id}
                    type="button"
                    role="tab"
                    aria-selected={item.id === modalityId}
                    className={item.id === modalityId ? "is-active" : ""}
                    mode={item.id === modalityId ? "primary" : "tertiary"}
                    appearance={item.id === modalityId ? "accent" : "neutral"}
                    size="m"
                    onClick={() => changeModality(item.id)}
                  >
                    {item.label}
                  </Button>
                ))}
              </div>
            </div>

            <div className="workflow-field">
              <label htmlFor="workflow-model">Модель</label>
              <NativeSelect
                id="workflow-model"
                className="workflow-select"
                value={modelId}
                onChange={(event) => setModelId(event.target.value)}
              >
                {currentModality.models.map((model) => (
                  <option key={model.id} value={model.id}>
                    {model.label}
                  </option>
                ))}
              </NativeSelect>
            </div>

            <div className="workflow-field">
              <label htmlFor="workflow-prompt">Промпт</label>
              <Textarea
                id="workflow-prompt"
                className="workflow-textarea"
                value={prompt}
                maxLength={PROMPT_LIMIT + 100}
                onChange={(event) => setPrompt(event.target.value)}
                rows={6}
                placeholder="Что нужно получить для VK?"
              />
              <span className={promptTooLong ? "field-note is-warn" : "field-note"}>
                {prompt.length.toLocaleString("ru-RU")} / {PROMPT_LIMIT.toLocaleString("ru-RU")}
              </span>
            </div>

            <div className="estimate-card" aria-live="polite">
              {estimateLoading ? (
                <span>Считаем стоимость...</span>
              ) : estimate ? (
                <>
                  <span>Стоимость</span>
                  <strong>{estimate.cost_estimate.toLocaleString("ru-RU")} кр.</strong>
                  {estimate.enough_credits ? <em>Кредитов достаточно</em> : <em className="is-warn">Недостаточно кредитов</em>}
                </>
              ) : estimateError ? (
                <em className="is-warn">{estimateError}</em>
              ) : (
                <span>Стоимость появится после промпта</span>
              )}
            </div>

            {submitError && <div className="workflow-error">{submitError}</div>}

            <Button
              type="button"
              className="workflow-primary"
              mode="primary"
              size="l"
              onClick={submitWorkflow}
              disabled={!canSubmit}
            >
              Запустить генерацию
            </Button>
          </div>
        </section>
      )}

      {screen === "status" && activeJob && (
        <StatusScreen job={activeJob} onResult={() => setScreen("result")} />
      )}

      {screen === "result" && activeJob && activeMessage && (
        <section className="workflow-screen">
          <ScreenTitle eyebrow="Result" title="Пост готов к проверке" text="Preview показывает контент так, как он будет ощущаться в VK." />
          <ResultCard msg={activeMessage} prompt={activePrompt} onRetry={submitWorkflow} />
          <Button
            type="button"
            className="quiet-action quiet-action--wide"
            mode="secondary"
            appearance="neutral"
            size="m"
            stretched
            onClick={() => setScreen("history")}
          >
            Перейти в историю
          </Button>
        </section>
      )}

      {screen === "history" && (
        <section className="workflow-screen">
          <ScreenTitle eyebrow="History" title="История генераций" text="Список приходит с backend; локальная очистка не удаляет jobs." />
          <div className="filter-row">
            <NativeSelect value={modalityFilter} onChange={(event) => setModalityFilter(event.target.value as "all" | ModalityId)} aria-label="Фильтр типа">
              <option value="all">Все типы</option>
              {MODALITIES.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.label}
                </option>
              ))}
            </NativeSelect>
            <NativeSelect value={statusFilter} onChange={(event) => setStatusFilter(event.target.value as typeof statusFilter)} aria-label="Фильтр статуса">
              {HISTORY_STATUS_FILTERS.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.label}
                </option>
              ))}
            </NativeSelect>
          </div>
          {filteredJobs.length === 0 ? (
            <div className="workflow-empty">Нет генераций под этот фильтр.</div>
          ) : (
            <JobList
              jobs={filteredJobs}
              onOpen={(job) => {
                setActiveJobId(job.id);
                setScreen(isTerminal(job.status) && statusKind(job.status) === "done" ? "result" : "status");
              }}
              onRepeat={repeatJob}
            />
          )}
          <Button
            type="button"
            className="quiet-action quiet-action--wide"
            mode="secondary"
            appearance="neutral"
            size="m"
            stretched
            onClick={onClearLocalHistory}
          >
            Очистить локальную историю
          </Button>
        </section>
      )}
    </main>
  );
}

function ScreenTitle({ eyebrow, title, text }: { eyebrow: string; title: string; text: string }) {
  return (
    <header className="screen-title">
      <span>{eyebrow}</span>
      <h1>{title}</h1>
      <p>{text}</p>
    </header>
  );
}

function JobList({
  jobs,
  onOpen,
  onRepeat,
}: {
  jobs: Job[];
  onOpen: (job: Job) => void;
  onRepeat?: (job: Job) => void;
}) {
  return (
    <div className="job-list">
      {jobs.map((job) => {
        const kind = statusKind(job.status);
        return (
          <article key={job.id} className="job-row">
            <button type="button" className="job-row__main" onClick={() => onOpen(job)}>
              <span>{operationLabel(job.operation)}</span>
              <strong>{kind === "done" ? "Готово" : kind === "failed" ? "Ошибка" : statusLabel(job.status)}</strong>
              <small>{dateLabel(job.created_at)}</small>
            </button>
            {onRepeat && (
              <Button
                type="button"
                className="job-row__repeat"
                mode="secondary"
                appearance="neutral"
                size="m"
                onClick={() => onRepeat(job)}
                disabled={!job.prompt}
              >
                Repeat
              </Button>
            )}
          </article>
        );
      })}
    </div>
  );
}

function StatusScreen({ job, onResult }: { job: Job; onResult: () => void }) {
  const active = timelineIndex(job.status);
  const failed = statusKind(job.status) === "failed";
  return (
    <section className="workflow-screen">
      <ScreenTitle eyebrow="Status" title={failed ? "Нужен повторный запуск" : "Генерация идёт"} text={failed ? "Кредиты будут обработаны backend-сценарием." : "Каждый этап обновляется из backend job state."} />
      <ol className="status-timeline" aria-live="polite">
        {TIMELINE.map((stage, index) => {
          const state = failed && index === TIMELINE.length - 1 ? " is-error" : index < active ? " is-done" : index === active ? " is-active" : "";
          return (
            <li key={stage.id} className={"status-step" + state}>
              <span className="status-step__dot" />
              <div>
                <strong>{stage.title}</strong>
                <p>{stage.text}</p>
              </div>
            </li>
          );
        })}
      </ol>
      {isTerminal(job.status) && statusKind(job.status) === "done" && (
        <Button
          type="button"
          className="workflow-primary"
          mode="primary"
          size="l"
          onClick={onResult}
        >
          Смотреть результат
        </Button>
      )}
    </section>
  );
}
