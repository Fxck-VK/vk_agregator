const SLUG_MAX = 24;

function extensionFromMime(mime: string): string {
  const normalized = mime.toLowerCase();
  if (normalized.includes("png")) return "png";
  if (normalized.includes("webp")) return "webp";
  if (normalized.includes("jpeg") || normalized.includes("jpg")) return "jpg";
  if (normalized.includes("mp4")) return "mp4";
  if (normalized.includes("webm")) return "webm";
  return "bin";
}

function slugFromPrompt(prompt: string): string {
  const normalized = prompt
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s_-]/gu, "")
    .replace(/\s+/g, "_")
    .replace(/_+/g, "_")
    .replace(/^_|_$/g, "")
    .slice(0, SLUG_MAX);
  return normalized;
}

/** Neirohub_{context}.{ext} or Neirohub_{jobIdPrefix}.{ext} when prompt is empty. */
export function neirohubArtifactFilename(
  prompt: string,
  jobId: string,
  mimeType?: string,
  fallbackExt = "png",
): string {
  const slug = slugFromPrompt(prompt);
  const shortId = jobId.replace(/-/g, "").slice(0, 8);
  const base = slug ? `Neirohub_${slug}` : `Neirohub_${shortId}`;
  const ext = mimeType ? extensionFromMime(mimeType) : fallbackExt;
  return `${base}.${ext}`;
}

export async function downloadFromBlobUrl(url: string, filename: string): Promise<void> {
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error("download failed");
  }
  const blob = await response.blob();
  const objectUrl = URL.createObjectURL(blob);
  try {
    const anchor = document.createElement("a");
    anchor.href = objectUrl;
    anchor.download = filename;
    anchor.rel = "noopener";
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
  } finally {
    URL.revokeObjectURL(objectUrl);
  }
}
