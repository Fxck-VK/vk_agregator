// src/chat/MessageBubble.tsx
import { Avatar, TypingDots } from "../ui/ui";
import { artifactUrl, statusLabel } from "../api/client";
import type { ChatMessage } from "./types";
import neuroHubAvatar from "../assets/neurohub-avatar.png";

function initials(name: string): string {
  const p = name.trim().split(/\s+/);
  return ((p[0]?.[0] ?? "") + (p[1]?.[0] ?? "")).toUpperCase() || "Я";
}

function BotContent({ msg }: { msg: ChatMessage }) {
  if (msg.error) return <span className="bubble__err">{msg.error}</span>;
  if (msg.pending) return <TypingDots />;

  const ids = msg.artifactIds ?? [];
  if (msg.operation === "image_generate" && ids.length > 0) {
    const src = artifactUrl(ids[0]);
    if (src) return <img className="bubble__media" src={src} alt="" />;
  }
  if (msg.operation === "video_generate" && ids.length > 0) {
    const src = artifactUrl(ids[0]);
    if (src) return <video className="bubble__media" src={src} controls />;
  }
  if (msg.text) return <span>{msg.text}</span>;
  if (ids.length > 0) return <span>Готово. Результат во вложении.</span>;
  return <span>Готово</span>;
}

export function MessageBubble({
  msg,
  userName,
  userAvatar,
  botName = "НейроХаб",
  onRetry,
}: {
  msg: ChatMessage;
  userName: string;
  userAvatar: string | null;
  botName?: string;
  onRetry?: () => void;
}) {
  const isUser = msg.role === "user";
  const showStatus = !isUser && msg.pending && !!msg.status;
  const author = isUser ? userName : botName;

  return (
    <div className={"msg " + (isUser ? "msg--user" : "msg--bot")}>
      <Avatar
        src={isUser ? userAvatar : neuroHubAvatar}
        fallback={isUser ? initials(userName) : "НХ"}
        className={isUser ? "" : "avatar--bot"}
      />
      <div className="bubble-wrap">
        <div className="message-author">{author}</div>
        <div className={"bubble " + (isUser ? "bubble--user" : "bubble--bot")}>
          {isUser ? <span>{msg.text}</span> : <BotContent msg={msg} />}
        </div>
        {showStatus && <div className="bubble__status">{statusLabel(msg.status!)}</div>}
        {!isUser && msg.error && onRetry && (
          <button type="button" className="bubble__retry" onClick={onRetry}>
            Повторить
          </button>
        )}
      </div>
    </div>
  );
}
