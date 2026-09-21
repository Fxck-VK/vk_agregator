// Dev fixture IDs encode only bounded dimensions, never user paths or image data.
export function createPreviewImage(aspectRatio: string) {
  const [a, b] = aspectRatio.split(":").map(Number);
  const scale = Math.floor(768 / Math.max(a, b));
  const width = a * scale;
  const height = b * scale;
  const id = `f1000000-${width.toString(16).padStart(4, "0")}-4${height.toString(16).padStart(3, "0")}-${crypto.randomUUID().slice(19)}`;
  if (!decodePreviewImageID(id)) throw new Error("Invalid preview dimensions");
  return { id, width, height, mime_type: "image/jpeg", size_bytes: 1 };
}

export function decodePreviewImageID(id: string) {
  const match = /^f1000000-([0-9a-f]{4})-4([0-9a-f]{3})-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.exec(id);
  if (!match) return null;
  const width = parseInt(match[1], 16);
  const height = parseInt(match[2], 16);
  return width >= 16 && height >= 16 && width <= 768 && height <= 768 ? { width, height } : null;
}
