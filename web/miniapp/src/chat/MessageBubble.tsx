// src/chat/MessageBubble.tsx
import { Avatar, TypingDots } from "../ui/ui";
import { useArtifactMediaUrl } from "../hooks/useArtifactMediaUrl";
import type { ChatMessage } from "./types";
import neuroHubAvatar from "../assets/neurohub-avatar.png";

function initials(name: string): string {
  const p = name.trim().split(/\s+/);
  return ((p[0]?.[0] ?? "") + (p[1]?.[0] ?? "")).toUpperCase() || "Я";
}

function BotContent({ msg }: { msg: ChatMessage }) {
  const mediaId = msg.artifactIds?.[0];
  const mediaSrc = useArtifactMediaUrl(mediaId);

  if (msg.error) return <span className="bubble__err">{msg.error}</span>;
  if (msg.pending) return <TypingDots />;

  if (msg.operation === "image_generate" && mediaSrc) {
    return <img className="bubble__media" src={mediaSrc} alt="" />;
  }
  if (msg.operation === "video_generate" && mediaSrc) {
    return <video className="bubble__media" src={mediaSrc} controls />;
  }
  if (msg.text) return <span>{msg.text}</span>;
  if (mediaId) return <span>Готово. Результат во вложении.</span>;
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
  const author = isUser ? "Вы" : botName;

  return (
    <div className={"msg " + (isUser ? "msg--user" : "msg--bot")}>
      <Avatar
        src={isUser ? userAvatar : neuroHubAvatar}
        fallback={isUser ? initials(userName) : "НХ"}
        className={isUser ? "" : "avatar--bot"}
      />
      <div className="bubble-wrap">
        <div className="message-author">{author}</div>
        <div
          className={
            "bubble " +
            (isUser ? "bubble--user" : "bubble--bot") +
            (!isUser && msg.pending ? " bubble--pending" : "")
          }
        >
          {isUser ? <span>{msg.text}</span> : <BotContent msg={msg} />}
        </div>
        {!isUser && msg.error && onRetry && (
          <button type="button" className="bubble__retry" onClick={onRetry}>
            Повторить
          </button>
        )}
      </div>
    </div>
  );
}
