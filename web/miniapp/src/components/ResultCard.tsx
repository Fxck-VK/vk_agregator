import { useMemo, useState } from "react";
import { artifactUrl, statusKind, statusLabel } from "../api/client";
import type { ChatMessage } from "../chat/types";

type ResultCardProps = {
  msg: ChatMessage;
  prompt: string;
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

export function ResultCard({ msg, prompt, onRetry }: ResultCardProps) {
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
          <span className="result-card__eyebrow">Готовый VK-пост</span>
          <h2 className="result-card__title">Результат генерации</h2>
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

      {showResult && msg.operation === "text_generate" && (
        text ? (
          <div className="result-card__text">{text}</div>
        ) : (
          <div className="result-card__fallback">
            <p>Текст результата недоступен.</p>
            <button type="button" className="result-card__btn" onClick={onRetry} disabled={!prompt}>
              Повторить
            </button>
          </div>
        )
      )}

      {showResult && msg.operation === "image_generate" && (
        mediaSrc && !mediaFailed ? (
          <img
            className="result-card__media"
            src={mediaSrc}
            alt="Готовый результат"
            onError={() => setMediaFailed(true)}
          />
        ) : (
          <div className="result-card__fallback">
            <p>Изображение недоступно.</p>
            <button type="button" className="result-card__btn" onClick={onRetry} disabled={!prompt}>
              Повторить
            </button>
          </div>
        )
      )}

      {showResult && msg.operation === "video_generate" && (
        mediaSrc && !mediaFailed ? (
          <video
            className="result-card__media"
            src={mediaSrc}
            controls
            onError={() => setMediaFailed(true)}
          />
        ) : (
          <div className="result-card__fallback">
            <p>Видео недоступно.</p>
            <button type="button" className="result-card__btn" onClick={onRetry} disabled={!prompt}>
              Повторить
            </button>
          </div>
        )
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
