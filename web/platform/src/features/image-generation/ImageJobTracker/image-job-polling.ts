export const imageJobPollDelays = [1500, 3000, 6000, 12000, 15000] as const;

export function nextImageJobPollDelay(attempt: number): number {
  const index = Math.min(Math.max(0, attempt), imageJobPollDelays.length - 1);
  return imageJobPollDelays[index];
}
