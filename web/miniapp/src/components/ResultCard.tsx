import { useMemo, useState } from "react";
import { artifactUrl, statusKind, statusLabel } from "../api/client";
import type { ChatMessage } from "../chat/types";

type ResultCardProps = {
  msg: ChatMessage;
  prompt: string;
  authorName?: string;
  authorAvatar?: string | null;
  onRetry: () => void;
};

function canShowResult(msg: ChatMessage): boolean {
  return !!msg.status && statusKind(msg.status) === "done" && !msg.pending && !msg.error;
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

export function ResultCard({ msg, prompt, authorName = "НейроХаб", authorAvatar, onRetry }: ResultCardProps) {
  const [copied, setCopied] = useState(false);
  const [mediaFailed, setMediaFailed] = useState(false);
  const text = msg.text?.trim() ?? "";
  const firstArtifactId = msg.artifactIds?.[0] ?? "";
  const mediaSrc = useMemo(
    () => (firstArtifactId ? artifactUrl(firstArtifactId) : null),
    [firstArtifactId],
  );
  const showResult = canShowResult(msg);
  const failed = !!msg.error || (!!msg.status && statusKind(msg.status) === "failed");

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
          <button type="button" className="result-card__btn" onClick={onRetry} disabled={!prompt}>
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

          {msg.operation === "text_generate" && (
            text ? (
              <div className="vk-preview__text">{text}</div>
            ) : (
              <UnavailableResult label="Текст результата недоступен." onRetry={onRetry} prompt={prompt} />
            )
          )}

          {msg.operation === "image_generate" && (
            mediaSrc && !mediaFailed ? (
              <img
                className="vk-preview__media"
                src={mediaSrc}
                alt="Готовый результат"
                onError={() => setMediaFailed(true)}
              />
            ) : (
              <UnavailableResult label="Изображение недоступно." onRetry={onRetry} prompt={prompt} />
            )
          )}

          {msg.operation === "video_generate" && (
            mediaSrc && !mediaFailed ? (
              <video
                className="vk-preview__media"
                src={mediaSrc}
                controls
                onError={() => setMediaFailed(true)}
              />
            ) : (
              <UnavailableResult label="Видео недоступно." onRetry={onRetry} prompt={prompt} />
            )
          )}
        </div>
      )}

      {showResult && msg.operation !== "text_generate" && msg.operation !== "image_generate" && msg.operation !== "video_generate" && (
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
          <button type="button" className="result-card__btn" onClick={onRetry} disabled={!prompt}>
            Повторить
          </button>
        </footer>
      )}
    </article>
  );
}

function UnavailableResult({ label, onRetry, prompt }: { label: string; onRetry: () => void; prompt: string }) {
  return (
    <div className="result-card__fallback">
      <p>{label}</p>
      <button type="button" className="result-card__btn" onClick={onRetry} disabled={!prompt}>
        Повторить
      </button>
    </div>
  );
}
