import { z } from "zod";
import type { ConversationItem, ConversationMessage, ConversationMessageList, ConversationImage, ImageModel } from "@/lib/web-api/contracts";
import { createPreviewImage } from "./preview-image";
import type { BrowserRequestInit } from "@/lib/web-api/development-transport";
import type { WebApiPath } from "@/lib/web-api/path";
import type { UploadScenario } from "./scenarios";

export type LocalConversationSeed = {
  conversations: readonly ConversationItem[];
  messages: Readonly<Record<string, ConversationMessageList>>;
  imageModels?: readonly ImageModel[];
};
const uuid = z.string().uuid();
const messageInput = z.object({
  prompt: z.string().trim().min(1).max(32_000),
  reference_artifact_ids: z.array(uuid).max(16).optional(),
  model_id: z.string().min(1).optional(),
  image_quality: z.string().min(1).optional(),
  aspect_ratio: z.string().regex(/^[1-9]\d?:[1-9]\d?$/).optional(),
  output_count: z.number().int().min(1).max(100).optional(),
}).passthrough();
const json = (data: unknown, status = 200) => Response.json(data, { status, headers: { "Cache-Control": "no-store" } });
type Job = { id: string; conversationID: string; readyAt: number; completed: boolean; reply: string; images?: ConversationImage[] };

export function createConversationSimulator({ seed, scenario, hasArtifact, reply, imageReply = reply }: {
  seed?: LocalConversationSeed;
  scenario: () => UploadScenario;
  hasArtifact: (id: string) => boolean;
  reply: () => string;
  imageReply?: () => string;
}) {
  const conversations = new Map((seed?.conversations ?? []).map(item => [item.id, { ...item }]));
  const messages = new Map(Object.entries(seed?.messages ?? {}).map(([id, page]) => [id, structuredClone(page.items)]));
  const jobs = new Map<string, Job>();
  const created = new Map<string, string>();
  const submitted = new Map<string, { body: string; job: Job }>();
  const now = () => new Date().toISOString();
  const jobResponse = (job: Job) => ({ job_id: job.id, status: job.completed ? "succeeded" : "queued" });
  const settle = () => {
    for (const job of jobs.values()) {
      if (job.completed || job.readyAt > Date.now()) continue;
      const history = messages.get(job.conversationID)!;
      history.push({ id: crypto.randomUUID(), seq: (history.at(-1)?.seq ?? 0) + 1, role: "assistant", text: job.reply, rating: null, created_at: now(), ...(job.images ? { images: job.images } : {}) });
      job.completed = true;
    }
  };
  function handle(path: WebApiPath, init: BrowserRequestInit = {}): Response | undefined {
    const url = new URL(path, "http://local-preview.invalid");
    const method = (init.method ?? "GET").toUpperCase();
    if (!url.pathname.startsWith("/web/v1/conversations")) return undefined;
    settle();
    if (url.pathname === "/web/v1/conversations") {
      if (method === "GET") return json({ items: [...conversations.values()] });
      if (method !== "POST") return undefined;
      const key = new Headers(init.headers).get("X-Idempotency-Key");
      if (!key || key.length > 256) return json({ error: "Missing idempotency key" }, 400);
      const known = created.get(key);
      if (known) return json(conversations.get(known));
      if (conversations.size >= 100) return json({ error: "Preview capacity reached" }, 429);
      const conversation: ConversationItem = { id: crypto.randomUUID(), title: "", created_at: now(), updated_at: now() };
      conversations.set(conversation.id, conversation); messages.set(conversation.id, []); created.set(key, conversation.id);
      return json(conversation, 201);
    }
    const match = url.pathname.match(/^\/web\/v1\/conversations\/([^/]+)(?:\/(messages|jobs)(?:\/([^/]+))?)?$/);
    if (!match) return undefined;
    const [, conversationID, resource, jobID] = match;
    const conversation = conversations.get(conversationID);
    if (!conversation) return json({ error: "Conversation not found" }, 404);
    if (!resource && method === "GET") return json(conversation);
    if (resource === "jobs" && method === "GET") {
      const job = jobs.get(jobID);
      return job?.conversationID === conversationID ? json(jobResponse(job)) : json({ error: "Job not found" }, 404);
    }
    if (resource !== "messages" || jobID) return undefined;
    const history = messages.get(conversationID) ?? [];
    if (method === "GET") {
      const after = Number(url.searchParams.get("after_seq") ?? 0);
      const before = Number(url.searchParams.get("before_seq") ?? Infinity);
      const limit = Math.max(1, Math.min(100, Number(url.searchParams.get("limit")) || 50));
      const filtered = history.filter(message => message.seq > after && message.seq < before);
      const items = after > 0 ? filtered.slice(0, limit) : filtered.slice(-limit);
      return json({ items, has_more_before: after === 0 && filtered.length > items.length });
    }
    if (method !== "POST") return undefined;
    const key = new Headers(init.headers).get("X-Idempotency-Key");
    if (!key || key.length > 256 || typeof init.body !== "string") return json({ error: "Invalid message request" }, 400);
    const keyScope = `${conversationID}:${key}`;
    const replay = submitted.get(keyScope);
    if (replay) return replay.body === init.body ? json(jobResponse(replay.job)) : json({ error: "Idempotency conflict" }, 409);
    let payload: z.infer<typeof messageInput>;
    try { payload = messageInput.parse(JSON.parse(init.body)); } catch { return json({ error: "Invalid message" }, 400); }
    const model = seed?.imageModels?.find(item => item.id === payload.model_id);
    const count = payload.output_count ?? 1;
    const ratio = payload.aspect_ratio ?? model?.default_aspect_ratio ?? "1:1";
    if (payload.image_quality && seed?.imageModels && (!model || count > (model.max_output_count ?? 1)
      || !model.quality_options.includes(payload.image_quality) || !model.allowed_aspect_ratios?.includes(ratio))) {
      return json({ error: "Invalid preview image options" }, 400);
    }
    const references = payload.reference_artifact_ids ?? [];
    if (references.some(id => !hasArtifact(id)) || new Set(references).size !== references.length) return json({ error: "Invalid attachments" }, 400);
    if (scenario() === "error") return json({ error: "Simulated message failure" }, 503);
    if (jobs.size >= 500) return json({ error: "Preview capacity reached" }, 429);
    const message: ConversationMessage = { id: crypto.randomUUID(), seq: (history.at(-1)?.seq ?? 0) + 1, role: "user", text: payload.prompt, rating: null, created_at: now(), ...(references.length ? { input_images: references } : {}) };
    const job: Job = { id: crypto.randomUUID(), conversationID, readyAt: Date.now() + (scenario() === "slow" ? 8000 : 1200), completed: false, reply: payload.image_quality ? imageReply() : reply() };
    if (payload.image_quality) {
      try {
        job.images = Array.from({ length: count }, () => ({
          job: { id: job.id, status: "succeeded", prompt: payload.prompt, model_id: payload.model_id ?? "preview",
            model_name: model?.name ?? payload.model_id ?? "Preview", image_quality: payload.image_quality!,
            cost_estimate: 20 * count, created_at: now(), updated_at: now() },
          artifact: createPreviewImage(ratio),
        }));
      } catch { return json({ error: "Invalid preview image dimensions" }, 400); }
    }
    history.push(message); messages.set(conversationID, history);
    jobs.set(job.id, job); submitted.set(keyScope, { body: init.body, job });
    conversation.updated_at = now(); if (!conversation.title) conversation.title = payload.prompt.slice(0, 80);
    return json(jobResponse(job), 201);
  }
  return {
    handle,
    isArtifactReferenced: (id: string) => [...messages.values()].some(history => history.some(message => message.input_images?.includes(id))),
    dispose() { conversations.clear(); messages.clear(); jobs.clear(); created.clear(); submitted.clear(); },
  };
}
