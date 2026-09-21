import "server-only";
import { join } from "node:path";
import sharp from "sharp";
import { decodePreviewImageID } from "./preview-image";

// Used only behind the server's local-workspace-preview gate. Never reads a
// client-supplied filename; repeated jobs share a bounded cache by dimensions.
const cache = new Map<string, Promise<Buffer>>();
export async function previewImageResponse(id: string): Promise<Response | undefined> {
  const dimensions = decodePreviewImageID(id);
  if (!dimensions) return undefined;
  const key = `${dimensions.width}:${dimensions.height}`;
  let pending = cache.get(key);
  if (!pending) {
    if (cache.size >= 32) cache.delete(cache.keys().next().value!);
    pending = sharp(join(process.cwd(), "public/assets/images/inspiration/paper-crane-cloud.png"))
      .resize({ ...dimensions, fit: "cover" }).jpeg({ quality: 85 }).toBuffer();
    cache.set(key, pending);
    pending.catch(() => cache.delete(key));
  }
  const bytes = await pending;
  return new Response(new Uint8Array(bytes), { headers: { "Content-Type": "image/jpeg", "Cache-Control": "no-store" } });
}
