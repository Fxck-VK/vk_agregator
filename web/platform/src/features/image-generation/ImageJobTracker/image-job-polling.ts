import type { ImageJob } from "@/lib/web-api/contracts";

export const imageJobPollDelays = [1500, 3000, 6000, 12000, 15000] as const;

const terminalImageJobStatuses = new Set<ImageJob["status"]>([
  "succeeded",
  "rejected",
  "failed_terminal",
  "cancelled",
  "expired",
  "refunded",
]);

export function nextImageJobPollDelay(attempt: number): number {
  const index = Math.min(Math.max(0, attempt), imageJobPollDelays.length - 1);
  return imageJobPollDelays[index];
}

export function isTerminalImageJobStatus(status: ImageJob["status"]): boolean {
  return terminalImageJobStatuses.has(status);
}
