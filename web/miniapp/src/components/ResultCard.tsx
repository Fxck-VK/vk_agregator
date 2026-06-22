import { useEffect, useState } from "react";
import { hasPreviewableMediaResult, statusKind, statusLabel, type Job } from "../api/client";
import { useArtifactMediaUrl } from "../hooks/useArtifactMediaUrl";
import type { ChatMessage } from "../chat/types";
import { MediaResultPreview } from "./MediaResultPreview";

type ResultCardProps = {
  msg: ChatMessage;
  prompt: string;
  authorName?: string;
  authorAvatar?: string | null;
  /** undefined — грузим сами; string — готовый URL; null — предзагрузка не удалась */
  mediaSrcOverride?: string | null;
  onRetry: () => void;
  retryDisabled?: boolean;
};

function canShowResult(msg: ChatMessage): boolean {
  if (msg.error || msg.pending || !msg.status) return false;
  if (statusKind(msg.status) === "done") return true;
  if (!msg.jobId) return false;
  const pseudoJob = {
    id: msg.jobId,
    operation: msg.operation ?? "",
    status: msg.status,
    output_artifact_ids: msg.artifactIds ?? [],
  } as Job;
  return hasPreviewableMediaResult(pseudoJob);
}

function safeStatus(msg: ChatMessage): string {
  if (msg.error) return "Не удалось";
  if (msg.status) return statusLabel(msg.status);
  return "Обработка";
}

function initials(name: string): string {
  const parts = name.trim().split(/\s+/);
  return ((parts[0]?.[0] ?? "") + (parts[1]?.[0] ?? "")).toUpperCase() || "НХ";
}

export function ResultCard({
  msg,
  prompt,
  authorName = "НейроХаб",
  authorAvatar,
  mediaSrcOverride,
  onRetry,
  retryDisabled = false,
}: ResultCardProps) {
  const [copied, setCopied] = useState(false);
  const [mediaFailed, setMediaFailed] = useState(false);
  const text = msg.text?.trim() ?? "";
  const firstArtifactId = msg.artifactIds?.[0];
  const overrideProvided = mediaSrcOverride !== undefined;
  const fetchedMediaSrc = useArtifactMediaUrl(overrideProvided ? undefined : firstArtifactId);
  const mediaSrc = overrideProvided ? (mediaSrcOverride ?? undefined) : fetchedMediaSrc;
  const preloadFailed = overrideProvided && mediaSrcOverride === null;
  const showResult = canShowResult(msg);
  const failed = !!msg.error || (!!msg.status && statusKind(msg.status) === "failed");
  const mediaLoading =
    showResult && Boolean(firstArtifactId) && !mediaSrc && !mediaFailed && !preloadFailed;

  useEffect(() => {
    setMediaFailed(false);
  }, [firstArtifactId, mediaSrc]);

  async function copyText() {
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    } catch {
      setCopied(false);
    }
  }

  return (
    <article className={"result-card" + (failed ? " result-card--error" : "")}>
      <header className="result-card__head">
        <div>
          <span className="result-card__eyebrow">Предпросмотр результата</span>
          <h2 className="result-card__title">Готовый результат</h2>
        </div>
        <span className="result-card__status">{safeStatus(msg)}</span>
      </header>

      {!showResult && !failed && (
        <div className="result-card__skeleton" aria-live="polite">
          <span />
          <span />
          <span />
        </div>
      )}

      {failed && (
        <div className="result-card__fallback">
          <p>{msg.error ?? "Не удалось выполнить запрос"}</p>
          <button type="button" className="result-card__btn" onClick={onRetry} disabled={retryDisabled}>
            Повторить
          </button>
        </div>
      )}

      {showResult && (
        <div className="vk-preview">
          <div className="vk-preview__meta">
            {authorAvatar ? (
              <img className="vk-preview__avatar" src={authorAvatar} alt="" />
            ) : (
              <span className="vk-preview__avatar" aria-hidden="true">
                {initials(authorName)}
              </span>
            )}
            <div>
              <strong>{authorName}</strong>
              <small>сейчас</small>
            </div>
          </div>

          {msg.operation === "text_generate" &&
            (text ? (
              <div className="vk-preview__text">{text}</div>
            ) : (
              <UnavailableResult label="Текст результата недоступен." onRetry={onRetry} prompt={prompt} />
            ))}

          {msg.operation === "image_generate" &&
            (mediaLoading ? (
              <div className="result-card__skeleton" aria-live="polite">
                <span />
                <span />
                <span />
              </div>
            ) : mediaSrc && !mediaFailed && msg.jobId ? (
              <MediaResultPreview
                kind="image"
                src={mediaSrc}
                prompt={prompt}
                jobId={msg.jobId}
                onMediaError={() => setMediaFailed(true)}
              />
            ) : (
              <UnavailableResult
                label={preloadFailed ? "Не удалось загрузить изображение." : "Изображение недоступно."}
                onRetry={onRetry}
                prompt={prompt}
              />
            ))}

          {msg.operation === "video_generate" &&
            (mediaLoading ? (
              <div className="result-card__skeleton" aria-live="polite">
                <span />
                <span />
                <span />
              </div>
            ) : mediaSrc && !mediaFailed && msg.jobId ? (
              <MediaResultPreview
                kind="video"
                src={mediaSrc}
                prompt={prompt}
                jobId={msg.jobId}
                onMediaError={() => setMediaFailed(true)}
              />
            ) : (
              <UnavailableResult
                label={preloadFailed ? "Не удалось загрузить видео." : "Видео недоступно."}
                onRetry={onRetry}
                prompt={prompt}
              />
            ))}
        </div>
      )}

      {showResult &&
        msg.operation !== "text_generate" &&
        msg.operation !== "image_generate" &&
        msg.operation !== "video_generate" && (
          <div className="result-card__fallback">
            <p>Результат готов.</p>
          </div>
        )}

      {showResult && (
        <footer className="result-card__actions">
          {text && (
            <button type="button" className="result-card__btn" onClick={copyText}>
              {copied ? "Скопировано" : "Копировать текст"}
            </button>
          )}
          <button type="button" className="result-card__btn" onClick={onRetry} disabled={retryDisabled}>
            Повторить
          </button>
        </footer>
      )}
    </article>
  );
}

function UnavailableResult({
  label,
  onRetry,
  retryDisabled = false,
}: {
  label: string;
  onRetry: () => void;
  prompt?: string;
  retryDisabled?: boolean;
}) {
  return (
    <div className="result-card__fallback">
      <p>{label}</p>
      <button type="button" className="result-card__btn" onClick={onRetry} disabled={retryDisabled}>
        Повторить
      </button>
    </div>
  );
}
