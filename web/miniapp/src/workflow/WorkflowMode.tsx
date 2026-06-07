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
import {
  type Chat,
  type ChatMessage,
  type ModalityId,
  modalityById,
  modalityByOperation,
} from "../chat/types";
import type { VkUser } from "../hooks/useBridge";
import neuroHubAvatar from "../assets/neurohub-avatar.png";

type WorkflowScreen = "home" | "generate" | "status" | "result" | "history";

type WorkflowModeProps = {
  user: VkUser;
  jobs: Job[];
  chats: Chat[];
  loading: boolean;
  submitting: boolean;
  onCreateJob: (prompt: string, request: { operation: string; modelId: string }) => Promise<Job | null>;
};

const ESTIMATE_DEBOUNCE_MS = 450;
const PROMPT_LIMIT = 2000;

const CREATE_CHOICES: Array<{
  title: string;
  modality: ModalityId;
}> = [
  {
    title: "Создать фото",
    modality: "image",
  },
  {
    title: "Создать видео",
    modality: "video",
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
    text: "Проверяем баланс и готовим запуск",
    statuses: new Set(["credits_reserved", "awaiting_payment"]),
  },
  {
    id: "queued",
    title: "Очередь",
    text: "Запрос ждёт своей очереди",
    statuses: new Set(["queued", "dispatching_provider"]),
  },
  {
    id: "generating",
    title: "Генерация",
    text: "НейроХаб готовит контент",
    statuses: new Set(["provider_submitted", "provider_pending", "provider_processing"]),
  },
  {
    id: "verifying",
    title: "Проверка результата",
    text: "Проверяем результат и готовим превью",
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
    artifactIds:
      isTerminal(job.status) && statusKind(job.status) === "done"
        ? job.output_artifact_ids
        : undefined,
    createdAt: job.created_at,
  };
}

function timelineIndex(status: string): number {
  if (statusKind(status) === "failed") return TIMELINE.length - 1;
  const index = TIMELINE.findIndex((stage) => stage.statuses.has(status));
  return index === -1 ? Math.max(0, TIMELINE.length - 2) : index;
}

export function WorkflowMode({
  user,
  jobs,
  chats,
  loading,
  submitting,
  onCreateJob,
}: WorkflowModeProps) {
  const [screen, setScreen] = useState<WorkflowScreen>("home");
  const [modalityId, setModalityId] = useState<ModalityId>("image");
  const [modelId, setModelId] = useState(modalityById("image").models[0].id);
  const [prompt, setPrompt] = useState("");
  const [estimate, setEstimate] = useState<EstimateResponse | null>(null);
  const [estimateLoading, setEstimateLoading] = useState(false);
  const [estimateError, setEstimateError] = useState<string | null>(null);
  const [activeJobId, setActiveJobId] = useState<string | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] =
    useState<(typeof HISTORY_STATUS_FILTERS)[number]["id"]>("all");

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

  function openCreateType(modality: ModalityId) {
    changeModality(modality);
    setPrompt("");
    setSubmitError(null);
    setStatusFilter("all");
    setScreen("generate");
  }

  function backToChoice() {
    setSubmitError(null);
    setScreen("home");
  }

  function openTypeHistory() {
    setStatusFilter("all");
    setScreen("history");
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

  const typeJobs = recentJobs.filter((job) => modalityByOperation(job.operation).id === modalityId);
  const filteredJobs = typeJobs.filter((job) => {
    const kind = statusKind(job.status);
    return statusFilter === "all" || statusFilter === kind;
  });

  return (
    <main className="workflow">
      {screen === "home" && (
        <section className="workflow-screen nh-scroll">
          <div className="nh-hero" aria-hidden="true">
            <img className="nh-hero__img" src={neuroHubAvatar} alt="" />
            <div className="nh-hero__fade" />
          </div>
          <div style={{ padding: "8px 16px 0" }}>
            <div style={{ marginBottom: "20px" }}>
              <h1 className="nh-page-title">Создать</h1>
              <p className="nh-page-sub">AI-генерация изображений и видео</p>
            </div>
            <section className="workflow-section" aria-label="Услуги">
              <div className="create-model-grid">
                {CREATE_CHOICES.map((choice) => {
                  const isImage = choice.modality === "image";
                  const color = isImage ? "#a855f7" : "#ec4899";
                  return (
                    <button
                      key={choice.title}
                      type="button"
                      className="create-model-card"
                      onClick={() => openCreateType(choice.modality)}
                    >
                      <div
                        className="create-model-card__icon"
                        style={{ background: `linear-gradient(135deg, ${color}, ${color}bb)` }}
                      >
                        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                          {isImage ? (
                            <path d="M4 5h16v14H4z M8 11l3 3 5-6" stroke="#fff" strokeWidth="1.8" />
                          ) : (
                            <path d="M4 6h16v12H4z M9 10l2 2 4-5" stroke="#fff" strokeWidth="1.8" />
                          )}
                        </svg>
                      </div>
                      <div style={{ fontSize: "13px", fontWeight: 600, marginBottom: "3px" }}>
                        {isImage ? "Stable Diffusion" : "Kling"}
                      </div>
                      <div style={{ fontSize: "11px", color: "var(--fg-muted)" }}>{choice.title}</div>
                    </button>
                  );
                })}
              </div>
            </section>
          </div>
        </section>
      )}

      {screen === "generate" && (
        <section className="workflow-screen">
          <WorkflowNav currentModality={currentModality.label} onBack={backToChoice} onHistory={openTypeHistory} />
          <ScreenTitle
            eyebrow="Заявка"
            title={`Опишите: ${currentModality.label.toLowerCase()}`}
            text="Мы заранее покажем стоимость и подскажем, можно ли запускать генерацию."
          />
          <div className="workflow-form">
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
              <label htmlFor="workflow-prompt">Описание</label>
              <Textarea
                id="workflow-prompt"
                className="workflow-textarea"
                value={prompt}
                maxLength={PROMPT_LIMIT + 100}
                onChange={(event) => setPrompt(event.target.value)}
                rows={6}
                placeholder="Опишите, что нужно получить"
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
                  {estimate.enough_credits ? (
                    <em>Кредитов достаточно</em>
                  ) : (
                    <em className="is-warn">Недостаточно кредитов</em>
                  )}
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
              Создать
            </Button>
          </div>

          <TypeHistoryPreview
            loading={loading}
            jobs={typeJobs}
            label={currentModality.label}
            onHistory={openTypeHistory}
            onOpen={(job) => {
              setActiveJobId(job.id);
              setScreen(isTerminal(job.status) && statusKind(job.status) === "done" ? "result" : "status");
            }}
          />
        </section>
      )}

      {screen === "status" && activeJob && (
        <>
          <WorkflowNav currentModality={currentModality.label} onBack={backToChoice} onHistory={openTypeHistory} />
          <StatusScreen job={activeJob} onResult={() => setScreen("result")} />
        </>
      )}

      {screen === "result" && activeJob && activeMessage && (
        <section className="workflow-screen">
          <WorkflowNav currentModality={currentModality.label} onBack={backToChoice} onHistory={openTypeHistory} />
          <ScreenTitle
            eyebrow="Результат"
            title="Результат готов к проверке"
            text="Посмотрите результат перед дальнейшим использованием."
          />
          <div className="workflow-signature-preview">
            <ResultCard
              msg={activeMessage}
              prompt={activePrompt}
              authorName={user.name}
              authorAvatar={user.avatar}
              onRetry={submitWorkflow}
            />
          </div>
          <Button
            type="button"
            className="quiet-action quiet-action--wide"
            mode="secondary"
            appearance="neutral"
            size="m"
            stretched
            onClick={openTypeHistory}
          >
            История этого типа
          </Button>
        </section>
      )}

      {screen === "history" && (
        <section className="workflow-screen">
          <WorkflowNav currentModality={currentModality.label} onBack={backToChoice} />
          <ScreenTitle
            eyebrow="История"
            title={`История: ${currentModality.label.toLowerCase()}`}
            text="Последние генерации выбранного формата."
          />
          <div className="filter-row filter-row--single">
            <NativeSelect
              value={statusFilter}
              onChange={(event) => setStatusFilter(event.target.value as typeof statusFilter)}
              aria-label="Фильтр статуса"
            >
              {HISTORY_STATUS_FILTERS.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.label}
                </option>
              ))}
            </NativeSelect>
          </div>
          {filteredJobs.length === 0 ? (
            <div className="workflow-empty">Нет генераций этого типа под выбранный фильтр.</div>
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
            onClick={() => setScreen("generate")}
          >
            Создать ещё
          </Button>
        </section>
      )}
    </main>
  );
}

function WorkflowNav({
  currentModality,
  onBack,
  onHistory,
}: {
  currentModality: string;
  onBack: () => void;
  onHistory?: () => void;
}) {
  return (
    <nav className="workflow-nav" aria-label="Create flow">
      <Button type="button" className="quiet-action" mode="secondary" appearance="neutral" size="m" onClick={onBack}>
        Назад
      </Button>
      <span>{currentModality}</span>
      {onHistory && (
        <Button type="button" className="quiet-action" mode="secondary" appearance="neutral" size="m" onClick={onHistory}>
          История
        </Button>
      )}
    </nav>
  );
}

function TypeHistoryPreview({
  loading,
  jobs,
  label,
  onHistory,
  onOpen,
}: {
  loading: boolean;
  jobs: Job[];
  label: string;
  onHistory: () => void;
  onOpen: (job: Job) => void;
}) {
  return (
    <section className="workflow-section" aria-labelledby="type-history-title">
      <div className="section-head">
        <h2 id="type-history-title">История: {label.toLowerCase()}</h2>
        <Button type="button" className="quiet-action" mode="secondary" appearance="neutral" size="m" onClick={onHistory}>
          Все
        </Button>
      </div>
      {loading ? (
        <div className="workflow-empty">Загружаем историю</div>
      ) : jobs.length === 0 ? (
        <div className="workflow-empty">Для этого типа пока нет результатов.</div>
      ) : (
        <JobList jobs={jobs.slice(0, 3)} onOpen={onOpen} />
      )}
    </section>
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
                Повторить
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
    <section className="workflow-screen workflow-screen--status">
      <ScreenTitle
        eyebrow="Статус"
        title={failed ? "Нужен повторный запуск" : "Генерация идёт"}
        text={failed ? "Если списание было, оно будет пересчитано автоматически." : "Статус обновится сам, когда результат будет готов."}
      />
      <ol className="status-timeline" aria-live="polite">
        {TIMELINE.map((stage, index) => {
          const state =
            failed && index === TIMELINE.length - 1
              ? " is-error"
              : index < active
                ? " is-done"
                : index === active
                  ? " is-active"
                  : "";
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
        <Button type="button" className="workflow-primary" mode="primary" size="l" onClick={onResult}>
          Смотреть результат
        </Button>
      )}
    </section>
  );
}
