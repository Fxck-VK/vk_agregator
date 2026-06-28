import { type Job } from "../api/client";
import { modalityByOperation } from "../chat/types";

const TITLE_MAX = 48;

export function truncateTitle(text: string, max = TITLE_MAX): string {
  const trimmed = text.trim();
  if (!trimmed) return "";
  return trimmed.length > max ? `${trimmed.slice(0, max)}…` : trimmed;
}

/** Contextual row title for image/video history lists. */
export function jobDisplayTitle(job: Job): string {
  const prompt = job.prompt?.trim();
  if (prompt) return truncateTitle(prompt);
  return modalityByOperation(job.operation).label;
}

/** Profile history: image/video jobs only. Chat/text history lives in Chat. */
export function dedupeHistoryJobs(jobs: Job[]): Job[] {
  return jobs
    .filter((job) => job.operation === "image_generate" || job.operation === "video_generate")
    .sort((a, b) => b.created_at.localeCompare(a.created_at));
}

export function historyCountLabel(count: number): string {
  const mod10 = count % 10;
  const mod100 = count % 100;
  if (mod10 === 1 && mod100 !== 11) return `${count} запись`;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return `${count} записи`;
  return `${count} записей`;
}
