import { describe, expect, it, vi } from "vitest";
import sharp from "sharp";
vi.mock("server-only", () => ({}));
import { imagePreviewResponse } from "./image-preview.server";

describe("private image previews", () => {
  it("shrinks an authorized image and keeps the aspect ratio without metadata or shared cache", async () => {
    const original = await sharp({ create: { width: 1800, height: 3200, channels: 3, background: "orange" } }).png().toBuffer();
    const response = await imagePreviewResponse(new Response(new Uint8Array(original), { headers: { "Content-Type": "image/png" } }));
    expect(response.status).toBe(200);
    expect(response.headers.get("Cache-Control")).toBe("private, no-store");
    const bytes = Buffer.from(await response.arrayBuffer());
    expect(bytes.length).toBeLessThan(original.length);
    const metadata = await sharp(bytes).metadata();
    expect(metadata).toMatchObject({ width: 360, height: 640, format: "webp" });
    expect(metadata.exif).toBeUndefined();
  });
  it("preserves authorization errors and rejects unsupported or oversized images", async () => {
    for (const status of [401, 403, 404]) {
      const denied = new Response(null, { status });
      expect(await imagePreviewResponse(denied)).toBe(denied);
    }
    expect((await imagePreviewResponse(new Response("<svg/>", { headers: { "Content-Type": "image/svg+xml" } }))).status).toBe(503);
    expect((await imagePreviewResponse(new Response("large", { headers: { "Content-Type": "image/png", "Content-Length": String(30 * 1024 * 1024) } }))).status).toBe(503);
  });
});
