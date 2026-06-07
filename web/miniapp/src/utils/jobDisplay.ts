import { type Job } from "../api/client";
import { modalityByOperation } from "../chat/types";

const TITLE_MAX = 48;

export function truncateTitle(text: string, max = TITLE_MAX): string {
  const trimmed = text.trim();
  if (!trimmed) return "";
  return trimmed.length > max ? `${trimmed.slice(0, max)}…` : trimmed;
}

/** Contextual row title for history lists (chat, image, video). */
export function jobDisplayTitle(job: Job): string {
  const prompt = job.prompt?.trim();
  if (prompt) return truncateTitle(prompt);
  return modalityByOperation(job.operation).label;
}

function conversationKey(job: Job): string {
  const id = job.conversation_id?.trim();
  if (id) return id;
  return `job:${job.id}`;
}

/**
 * Profile history: one row per chat thread (first user message), plus each image/video job.
 */
export function dedupeHistoryJobs(jobs: Job[]): Job[] {
  const mediaJobs: Job[] = [];
  const chatFirstByConversation = new Map<string, Job>();

  const chronological = [...jobs].sort((a, b) => a.created_at.localeCompare(b.created_at));
  for (const job of chronological) {
    if (job.operation === "text_generate") {
      const key = conversationKey(job);
      if (!chatFirstByConversation.has(key)) {
        chatFirstByConversation.set(key, job);
      }
      continue;
    }
    if (job.operation === "image_generate" || job.operation === "video_generate") {
      mediaJobs.push(job);
    }
  }

  return [...mediaJobs, ...chatFirstByConversation.values()].sort((a, b) =>
    b.created_at.localeCompare(a.created_at),
  );
}

export function historyCountLabel(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return `${count} запись`;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return `${count} записи`;
  return `${count} записей`;
}
