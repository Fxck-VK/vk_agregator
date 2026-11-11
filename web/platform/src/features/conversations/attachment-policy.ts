import type { GenerationModel } from "@/features/models/generation-model-catalog";
import type { PublicOperation } from "@/features/models/model-catalog-contract";

type Format = { mime: string; extension: string };
export type AttachmentPolicy = {
  enabled: boolean;
  accept: string;
  formats: readonly Format[];
  maxCount: number;
  maxBytes: number;
  maxTotalBytes: number;
  allowedCounts?: readonly number[];
  required: boolean;
};

const unavailable: AttachmentPolicy = {
  enabled: false, accept: "", formats: [], maxCount: 0, maxBytes: 0, maxTotalBytes: 0, required: false,
};

// The conversation reference transport accepts owned PNG/JPEG images only.
// Provider-native capabilities do not grant access to another upload transport.
const transportFormats = new Set(["image/png", "image/jpeg"]);

export function attachmentPolicy(model?: GenerationModel): AttachmentPolicy {
  const kind = model?.category === "images" ? "image" : model?.category;
  if (kind !== "image" && kind !== "video") return unavailable;
  const operation = (model?.operations as PublicOperation[] | undefined)?.find(op => op.enabled && op.kind === kind);
  const input = operation?.inputs.images;
  if (!input?.enabled || input.support !== "supported" || !input.max_count || !input.max_bytes) return unavailable;
  const formats = (input.formats ?? []).filter(format => transportFormats.has(format.mime));
  if (!formats.length) return unavailable;
  const accept = [...new Set(formats.flatMap(format => [format.mime, format.extension]))].join(",");
  return {
    enabled: true, accept, formats, maxCount: input.max_count, maxBytes: input.max_bytes,
    maxTotalBytes: operation!.inputs.max_total_bytes || input.max_count * input.max_bytes,
    allowedCounts: input.allowed_counts,
    required: input.required === true,
  };
}

export function acceptsAttachment(policy: AttachmentPolicy, item: { mimeType: string; name: string; file?: File }) {
  if (!policy.enabled) return false;
  // An empty native MIME may use the admitted extension; a conflicting MIME may not.
  if (item.file?.type === "" && (item.mimeType === "application/octet-stream" || item.mimeType === "")) return policy.formats.some(format => item.name.toLowerCase().endsWith(format.extension));
  return policy.formats.some(format => format.mime === item.mimeType);
}

export function attachmentCountAllowed(policy: AttachmentPolicy, count: number) {
  if (count === 0) return !policy.required && (!policy.allowedCounts || policy.allowedCounts.includes(0));
  return policy.enabled && count <= policy.maxCount && (!policy.allowedCounts || policy.allowedCounts.includes(count));
}
