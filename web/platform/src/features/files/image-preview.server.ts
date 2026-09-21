import "server-only";
import sharp from "sharp";

const maxBytes = 20 * 1024 * 1024;
let active = 0;
const waiting: Array<() => void> = [];
const headers = { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" };
const failure = () => new Response(null, { status: 503, headers });

function acquire(signal?: AbortSignal): Promise<boolean> {
  if (signal?.aborted || waiting.length >= 32) return Promise.resolve(false);
  if (active < 4) { active++; return Promise.resolve(true); }
  return new Promise(resolve => {
    const start = () => { signal?.removeEventListener("abort", cancel); active++; resolve(true); };
    const cancel = () => {
      const index = waiting.indexOf(start);
      if (index >= 0) waiting.splice(index, 1);
      resolve(false);
    };
    signal?.addEventListener("abort", cancel, { once: true });
    waiting.push(start);
  });
}

/** The source must come from the authenticated artifact proxy (including its attested redirect). */
export async function imagePreviewResponse(source: Response, signal?: AbortSignal): Promise<Response> {
  if (source.status !== 200 && source.status !== 307) return source;
  if (!await acquire(signal)) { void source.body?.cancel(); return failure(); }
  let reader: ReadableStreamDefaultReader<Uint8Array> | undefined;
  try {
    if (source.status === 307) {
      const location = source.headers.get("Location");
      if (!location) return failure();
      source = await fetch(location, { redirect: "error", cache: "no-store", signal: signal ? AbortSignal.any([signal, AbortSignal.timeout(12_000)]) : AbortSignal.timeout(12_000) });
    }
    if (source.status !== 200 || !/^image\/(png|jpeg|webp|avif)(;|$)/i.test(source.headers.get("Content-Type") ?? "")) return failure();
    if (Number(source.headers.get("Content-Length")) > maxBytes || !source.body) return failure();
    reader = source.body.getReader();
    const chunks: Uint8Array[] = [];
    let length = 0;
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      length += value.byteLength;
      if (length > maxBytes || signal?.aborted) { await reader.cancel(); return failure(); }
      chunks.push(value);
    }
    const bytes = await sharp(Buffer.concat(chunks), { limitInputPixels: 40_000_000 })
      .rotate().resize(640, 640, { fit: "inside", withoutEnlargement: true })
      .webp({ quality: 78 }).timeout({ seconds: 5 }).toBuffer();
    return new Response(new Uint8Array(bytes), { headers: { ...headers, "Content-Type": "image/webp" } });
  } catch { return failure(); }
  finally {
    if (reader) { await reader.cancel().catch(() => undefined); reader.releaseLock(); }
    else if (source.body && !source.body.locked) await source.body.cancel().catch(() => undefined);
    active -= 1;
    waiting.shift()?.();
  }
}
