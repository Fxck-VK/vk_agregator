import { z } from "zod";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";

export type SpeechRequest = {
  model_id: "gpt_4o_mini_tts" | "whisper_1";
  speech: { text?: string; voice?: string; format: "wav" | "json"; speed?: number; language?: string };
  audio_artifact_id?: string;
};
const jobSchema = z.object({ id: z.uuid(), model_id: z.string(), status: z.string(), cost_estimate: z.number().int().nonnegative(), created_at: z.string() }).strict();
const preparationSchema = z.object({ job: jobSchema, balance: z.number().nonnegative(), can_afford: z.boolean() }).strict();
export type SpeechPreparation = z.infer<typeof preparationSchema>;
const artifactSchema = z.object({ id: z.uuid(), kind: z.string(), format: z.string().optional(), url: z.string().regex(/^\/web\/v1\/speech-artifacts\/[0-9a-f-]{36}$/) }).strict();
export type SpeechArtifact = z.infer<typeof artifactSchema>;

export async function prepareSpeech(request: SpeechRequest, key: string) {
  const response = await webBrowserMutation("/web/v1/speech-jobs/prepare", { method: "POST", headers: { "Content-Type": "application/json", "X-Idempotency-Key": key }, body: JSON.stringify(request) });
  if (response.status !== 201) throw new Error("Не удалось подготовить задание. Списание не выполнено.");
  return preparationSchema.parse(await response.json());
}
export async function activateSpeech(id: string) {
  const response = await webBrowserMutation(`/web/v1/speech-jobs/${id}/activate`, { method: "POST", headers: { "X-Idempotency-Key": id } });
  if (response.status !== 200) throw new Error("Не удалось подтвердить запуск. Повторите подтверждение этого задания.");
  return z.object({ job: jobSchema }).strict().parse(await response.json()).job;
}
export async function loadSpeechJob(id: string) {
  const response = await webBrowserFetch(`/web/v1/speech-jobs/${id}`);
  if (!response.ok) throw new Error("Не удалось проверить задание.");
  return z.object({ job: jobSchema }).strict().parse(await response.json()).job;
}
export async function loadSpeechResult(id: string) {
  const response = await webBrowserFetch(`/web/v1/speech-jobs/${id}/result`);
  if (!response.ok) throw new Error("Результат пока недоступен.");
  return z.object({ job_id: z.uuid(), artifacts: z.array(artifactSchema) }).strict().parse(await response.json()).artifacts;
}
