import { z } from "zod";

const conversationIDSchema = z.string().uuid();
const schema = z.object({
  jobID: z.string().uuid(),
  baselineSeq: z.number().int().nonnegative(),
  startedAt: z.number().int().positive(),
}).strict();
const maxPendingTextJobAgeMs = 60 * 60 * 1_000;
const key = (conversationID: string) => `neirohub:conversation-pending-text:${conversationID}`;

export function readPendingTextJob(conversationID: string) {
  if (!conversationIDSchema.safeParse(conversationID).success) return null;
  try {
    const raw = window.sessionStorage.getItem(key(conversationID));
    if (raw === null) return null;
    const result = schema.safeParse(JSON.parse(raw));
    const age = result.success ? Date.now() - result.data.startedAt : -1;
    if (result.success && age >= 0 && age < maxPendingTextJobAgeMs) return result.data;
    clearPendingTextJob(conversationID);
  } catch {
    clearPendingTextJob(conversationID);
  }
  return null;
}

export function savePendingTextJob(conversationID: string, jobID: string, baselineSeq: number) {
  if (!conversationIDSchema.safeParse(conversationID).success) return;
  const result = schema.safeParse({ jobID, baselineSeq, startedAt: Date.now() });
  if (!result.success) return;
  try {
    window.sessionStorage.setItem(key(conversationID), JSON.stringify(result.data));
  } catch { /* Optional recovery metadata must not affect an accepted message. */ }
}

export function clearPendingTextJob(conversationID: string) {
  try {
    window.sessionStorage.removeItem(key(conversationID));
  } catch { /* Browser storage may be unavailable. */ }
}
