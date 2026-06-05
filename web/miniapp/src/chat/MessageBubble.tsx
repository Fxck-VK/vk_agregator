// src/chat/MessageBubble.tsx
import { Avatar, TypingDots } from "../ui/ui";
import { artifactUrl, statusLabel } from "../api/client";
import type { ChatMessage } from "./types";

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
}: {
  msg: ChatMessage;
  userName: string;
  userAvatar: string | null;
}) {
  const isUser = msg.role === "user";
  const showStatus = !isUser && msg.pending && !!msg.status;

  return (
    <div className={"msg " + (isUser ? "msg--user" : "msg--bot")}>
      <Avatar
        src={isUser ? userAvatar : null}
        fallback={isUser ? initials(userName) : "AI"}
      />
      <div className="bubble-wrap">
        <div className={"bubble " + (isUser ? "bubble--user" : "bubble--bot")}>
          {isUser ? <span>{msg.text}</span> : <BotContent msg={msg} />}
        </div>
        {showStatus && <div className="bubble__status">{statusLabel(msg.status!)}</div>}
      </div>
    </div>
  );
}
