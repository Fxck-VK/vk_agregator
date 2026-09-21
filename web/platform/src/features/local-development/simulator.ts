import type { BrowserRequestInit, DevelopmentTransport } from "@/lib/web-api/development-transport";
import { WebNetworkError } from "@/lib/web-api/network-error";
import type { LocalScenarios } from "./scenarios";
import { createConversationSimulator, type LocalConversationSeed } from "./conversation-simulator";

const maxFileBytes = 20 * 1024 * 1024;
const maxRetainedBytes = 64 * 1024 * 1024;
const json = (payload: unknown, status = 200) => Response.json(payload, { status, headers: { "Cache-Control": "no-store" } });

function wait(ms: number, signal: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    if (signal.aborted) { reject(signal.reason); return; }
    const abort = () => { clearTimeout(timer); reject(signal.reason); };
    const timer = setTimeout(() => { signal.removeEventListener("abort", abort); resolve(); }, ms);
    signal.addEventListener("abort", abort, { once: true });
  });
}

// No fetch/XHR, credentials, persistent file storage or provider calls here.
// Adding future scenarios belongs at this request boundary, not in UI hooks.
export function createLocalSimulator(getScenarios: () => LocalScenarios, options?: { seed?: LocalConversationSeed; reply?: () => string; imageReply?: () => string }) {
  const pending = new Set<AbortController>();
  const artifacts = new Map<string, { file: File; mime: string }>();
  const conversations = createConversationSimulator({
    seed: options?.seed,
    scenario: () => getScenarios().message,
    hasArtifact: id => artifacts.has(id),
    reply: options?.reply ?? (() => ""),
    imageReply: options?.imageReply,
  });
  let retainedBytes = 0;

  async function upload(init: BrowserRequestInit, scenario: LocalScenarios["upload"]) {
    const controller = new AbortController();
    const cancel = () => controller.abort();
    pending.add(controller);
    init.signal?.addEventListener("abort", cancel, { once: true });
    if (init.signal?.aborted) cancel();
    const { signal } = controller;
    try {
      signal.throwIfAborted();
      const file = init.body instanceof FormData ? init.body.get("file") : null;
      if (!(file instanceof File) || file.size === 0) return json({ error: "Invalid image" }, 400);
      if (file.size > maxFileBytes) return json({ error: "Image too large" }, 413);
      const mime = file.type || (/\.png$/i.test(file.name) ? "image/png" : /\.jpe?g$/i.test(file.name) ? "image/jpeg" : "");
      if (!["image/png", "image/jpeg"].includes(mime)) return json({ error: "Invalid image" }, 400);
      let dimensions: { width: number; height: number };
      try {
        const bitmap = await createImageBitmap(file);
        dimensions = { width: bitmap.width, height: bitmap.height };
        bitmap.close();
      } catch {
        signal.throwIfAborted();
        return json({ error: "Invalid image" }, 400);
      }
      signal.throwIfAborted();
      init.onUploadProgress?.(0);
      if (scenario === "offline") {
        await wait(150, signal);
        throw new WebNetworkError();
      }
      for (let step = 1; step <= 10; step++) {
        await wait(scenario === "slow" ? 600 : 100, signal);
        signal.throwIfAborted();
        init.onUploadProgress?.(step * 10);
        if (scenario === "error" && step === 6) return json({ error: "Simulated upload failure" }, 503);
      }
      // Exercise the processing state after 100% has been transferred.
      await wait(250, signal);
      const id = `f0000000-${crypto.randomUUID().slice(9)}`;
      while (artifacts.size >= 50 || retainedBytes + file.size > maxRetainedBytes) {
        const oldest = [...artifacts.entries()].find(([artifactID]) => !conversations.isArtifactReferenced(artifactID));
        if (!oldest) return json({ error: "Preview storage full" }, 507);
        retainedBytes -= oldest[1].file.size;
        artifacts.delete(oldest[0]);
      }
      artifacts.set(id, { file, mime }); retainedBytes += file.size;
      return json({ artifact_id: id, mime_type: mime, size_bytes: file.size, ...dimensions }, 201);
    } finally {
      pending.delete(controller);
      init.signal?.removeEventListener("abort", cancel);
    }
  }

  const handle: DevelopmentTransport = (path, init = {}) => {
    const scenarios = getScenarios();
    if (!scenarios.enabled) return undefined;
    if (init.signal?.aborted) return Promise.reject(new DOMException("Aborted", "AbortError"));
    const url = new URL(path, "http://local-preview.invalid");
    const method = (init.method ?? "GET").toUpperCase();
    if (url.pathname === "/web/v1/input-artifacts" && method === "POST") return upload(init, scenarios.upload);
    if (url.pathname === "/web/v1/image-reference-quote" && method === "GET") {
      const count = Number(url.searchParams.get("output_count") ?? 1);
      return Promise.resolve(scenarios.quote === "error" ? json({ error: "Simulated quote failure" }, 503)
        : json({ credits: 20 * (Number.isInteger(count) && count >= 1 && count <= 100 ? count : 1) }));
    }
    const match = url.pathname.match(/^\/web\/v1\/(?:input|image)-artifacts\/(f0000000-[a-f0-9-]+)$/);
    if (match && method === "GET") {
      const artifact = artifacts.get(match[1]);
      return Promise.resolve(artifact
        ? new Response(artifact.file, { headers: { "Content-Type": artifact.mime, "Cache-Control": "no-store" } })
        : json({ error: "Preview artifact expired" }, 404));
    }
    const conversation = conversations.handle(path, init);
    if (conversation) return Promise.resolve(conversation);
    if (!["GET", "HEAD", "OPTIONS"].includes(method)) return Promise.resolve(json({ error: "This action is not simulated" }, 503));
    return undefined;
  };

  return { handle, dispose() { pending.forEach(controller => controller.abort()); pending.clear(); artifacts.clear(); retainedBytes = 0; conversations.dispose(); } };
}
