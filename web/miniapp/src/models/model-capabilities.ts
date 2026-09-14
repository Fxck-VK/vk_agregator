export type InputCapability = {
  support: "supported" | "unsupported" | "unknown";
  extensions: string[] | null;
  max_count: number | null;
};
export type CapabilityProfile = {
  text?: { images: InputCapability; videos: InputCapability; files: InputCapability };
  image?: { images: InputCapability; aspect_ratios: string[] | null; resolutions: string[] | null; quality_modes: string[] | null; speed_modes: string[] | null; max_output_count: number | null; max_combined_images: number | null };
  video?: { images: InputCapability; videos: InputCapability; allowed_image_counts: number[] | null; duration: { mode: string; min_seconds: number | null; max_seconds: number | null; allowed_seconds: number[] | null; by_resolution?: Record<string, number[]>; max_by_orientation?: Record<string, number> }; resolutions: string[] | null; quality_modes: string[] | null; aspect_ratios: string[] | null; audio: { mode: string; selectable: boolean }; start_frame: string; end_frame: string };
  audio?: { audio: InputCapability; videos: InputCapability; output: string };
  notes?: string[];
};
export type ModelCapabilities = { schema_version: number; api: CapabilityProfile; application: CapabilityProfile };

type Purpose = "text" | "image" | "video" | "audio";

const MAX_CAPABILITY_ITEMS = 120;
const SUPPORT = new Set(["supported", "unsupported", "unknown"]);
const DURATION_MODES = new Set(["selected", "automatic", "reference_video", "unknown"]);
const VIDEO_AUDIO_MODES = new Set(["optional", "generated", "silent", "preserve_source", "unknown"]);
const FRAME_MODES = new Set(["required", "optional", "unsupported", "unknown"]);
const FORBIDDEN_DYNAMIC_KEYS = new Set(["__proto__", "constructor", "prototype"]);

export function parseModelCapabilities(value: unknown): ModelCapabilities | undefined {
  if (!isRecord(value) || !hasOnlyKeys(value, ["schema_version", "api", "application"])) return undefined;
  if (value.schema_version !== 1) return undefined;
  const api = parseProfile(value.api);
  const application = parseProfile(value.application);
  if (!api || !application || api.purpose !== application.purpose) return undefined;
  return { schema_version: 1, api: api.profile, application: application.profile };
}

function parseProfile(value: unknown): { profile: CapabilityProfile; purpose: Purpose } | undefined {
  if (!isRecord(value) || !hasOnlyKeys(value, ["text", "image", "video", "audio", "notes"])) return undefined;
  const purposes = (["text", "image", "video", "audio"] as const).filter((purpose) => value[purpose] !== undefined);
  if (purposes.length !== 1) return undefined;
  const purpose = purposes[0];
  const notes = value.notes === undefined ? undefined : parseStringArray(value.notes);
  if (value.notes !== undefined && notes === undefined) return undefined;

  switch (purpose) {
    case "text": {
      const text = parseTextProfile(value.text);
      if (!text) return undefined;
      return { purpose, profile: notes ? { text, notes } : { text } };
    }
    case "image": {
      const image = parseImageProfile(value.image);
      if (!image) return undefined;
      return { purpose, profile: notes ? { image, notes } : { image } };
    }
    case "video": {
      const video = parseVideoProfile(value.video);
      if (!video) return undefined;
      return { purpose, profile: notes ? { video, notes } : { video } };
    }
    case "audio": {
      const audio = parseAudioProfile(value.audio);
      if (!audio) return undefined;
      return { purpose, profile: notes ? { audio, notes } : { audio } };
    }
  }
}

function parseTextProfile(value: unknown): NonNullable<CapabilityProfile["text"]> | undefined {
  if (!isRecord(value) || !hasOnlyKeys(value, ["images", "videos", "files"])) return undefined;
  const images = parseInputCapability(value.images);
  const videos = parseInputCapability(value.videos);
  const files = parseInputCapability(value.files);
  if (!images || !videos || !files) return undefined;
  return { images, videos, files };
}

function parseImageProfile(value: unknown): NonNullable<CapabilityProfile["image"]> | undefined {
  if (
    !isRecord(value) ||
    !hasOnlyKeys(value, [
      "images",
      "aspect_ratios",
      "resolutions",
      "quality_modes",
      "speed_modes",
      "max_output_count",
      "max_combined_images",
    ])
  ) {
    return undefined;
  }
  const images = parseInputCapability(value.images);
  const aspect_ratios = parseStringArrayOrNull(value.aspect_ratios);
  const resolutions = parseStringArrayOrNull(value.resolutions);
  const quality_modes = parseStringArrayOrNull(value.quality_modes);
  const speed_modes = parseStringArrayOrNull(value.speed_modes);
  const max_output_count = parseMaximum(value.max_output_count);
  const max_combined_images = parseMaximum(value.max_combined_images);
  if (
    !images ||
    aspect_ratios === undefined ||
    resolutions === undefined ||
    quality_modes === undefined ||
    speed_modes === undefined ||
    max_output_count === undefined ||
    max_combined_images === undefined
  ) {
    return undefined;
  }
  return { images, aspect_ratios, resolutions, quality_modes, speed_modes, max_output_count, max_combined_images };
}

function parseVideoProfile(value: unknown): NonNullable<CapabilityProfile["video"]> | undefined {
  if (
    !isRecord(value) ||
    !hasOnlyKeys(value, [
      "images",
      "videos",
      "allowed_image_counts",
      "duration",
      "resolutions",
      "quality_modes",
      "aspect_ratios",
      "audio",
      "start_frame",
      "end_frame",
    ])
  ) {
    return undefined;
  }
  const images = parseInputCapability(value.images);
  const videos = parseInputCapability(value.videos);
  const allowed_image_counts = parseNumberArrayOrNull(value.allowed_image_counts);
  const duration = parseDuration(value.duration);
  const resolutions = parseStringArrayOrNull(value.resolutions);
  const quality_modes = parseStringArrayOrNull(value.quality_modes);
  const aspect_ratios = parseStringArrayOrNull(value.aspect_ratios);
  const audio = parseVideoAudio(value.audio);
  if (
    !images ||
    !videos ||
    allowed_image_counts === undefined ||
    !duration ||
    resolutions === undefined ||
    quality_modes === undefined ||
    aspect_ratios === undefined ||
    !audio ||
    typeof value.start_frame !== "string" ||
    !FRAME_MODES.has(value.start_frame) ||
    typeof value.end_frame !== "string" ||
    !FRAME_MODES.has(value.end_frame)
  ) {
    return undefined;
  }
  return {
    images,
    videos,
    allowed_image_counts,
    duration,
    resolutions,
    quality_modes,
    aspect_ratios,
    audio,
    start_frame: value.start_frame,
    end_frame: value.end_frame,
  };
}

function parseAudioProfile(value: unknown): NonNullable<CapabilityProfile["audio"]> | undefined {
  if (!isRecord(value) || !hasOnlyKeys(value, ["audio", "videos", "output"])) return undefined;
  const audio = parseInputCapability(value.audio);
  const videos = parseInputCapability(value.videos);
  if (!audio || !videos || typeof value.output !== "string") return undefined;
  return { audio, videos, output: value.output };
}

function parseInputCapability(value: unknown): InputCapability | undefined {
  if (!isRecord(value) || !hasOnlyKeys(value, ["support", "extensions", "max_count"])) return undefined;
  if (typeof value.support !== "string" || !SUPPORT.has(value.support)) return undefined;
  const extensions = parseStringArrayOrNull(value.extensions);
  const max_count = parseMaximum(value.max_count);
  if (extensions === undefined || max_count === undefined) return undefined;
  return { support: value.support as InputCapability["support"], extensions, max_count };
}

function parseDuration(value: unknown): NonNullable<CapabilityProfile["video"]>["duration"] | undefined {
  if (!isRecord(value) || !hasOnlyKeys(value, ["mode", "min_seconds", "max_seconds", "allowed_seconds", "by_resolution", "max_by_orientation"])) return undefined;
  if (typeof value.mode !== "string" || !DURATION_MODES.has(value.mode)) return undefined;
  const min_seconds = parseMaximum(value.min_seconds);
  const max_seconds = parseMaximum(value.max_seconds);
  const allowed_seconds = parseNumberArrayOrNull(value.allowed_seconds);
  const by_resolution = value.by_resolution === undefined ? undefined : parseNumberArrayRecord(value.by_resolution);
  const max_by_orientation = value.max_by_orientation === undefined ? undefined : parseNumberRecord(value.max_by_orientation);
  if (min_seconds === undefined || max_seconds === undefined || allowed_seconds === undefined || by_resolution === undefined && value.by_resolution !== undefined || max_by_orientation === undefined && value.max_by_orientation !== undefined) return undefined;
  return {
    mode: value.mode,
    min_seconds,
    max_seconds,
    allowed_seconds,
    ...(by_resolution ? { by_resolution } : {}),
    ...(max_by_orientation ? { max_by_orientation } : {}),
  };
}

function parseVideoAudio(value: unknown): NonNullable<CapabilityProfile["video"]>["audio"] | undefined {
  if (!isRecord(value) || !hasOnlyKeys(value, ["mode", "selectable"])) return undefined;
  if (typeof value.mode !== "string" || !VIDEO_AUDIO_MODES.has(value.mode) || typeof value.selectable !== "boolean") {
    return undefined;
  }
  return { mode: value.mode, selectable: value.selectable };
}

function parseStringArrayOrNull(value: unknown): string[] | null | undefined {
  if (value === null) return null;
  return parseStringArray(value);
}

function parseStringArray(value: unknown): string[] | undefined {
  if (!Array.isArray(value) || value.length > MAX_CAPABILITY_ITEMS || value.some((item) => typeof item !== "string")) {
    return undefined;
  }
  return [...value];
}

function parseNumberArrayOrNull(value: unknown): number[] | null | undefined {
  if (value === null) return null;
  if (!Array.isArray(value) || value.length > MAX_CAPABILITY_ITEMS || value.some((item) => !isNonnegativeInteger(item))) {
    return undefined;
  }
  return [...value];
}

function parseNumberArrayRecord(value: unknown): Record<string, number[]> | undefined {
  if (!isRecord(value)) return undefined;
  const entries = Object.entries(value);
  if (entries.length > MAX_CAPABILITY_ITEMS) return undefined;
  const parsedEntries: [string, number[]][] = [];
  for (const [key, item] of entries) {
    if (FORBIDDEN_DYNAMIC_KEYS.has(key)) return undefined;
    const parsed = parseNumberArrayOrNull(item);
    if (!parsed) return undefined;
    parsedEntries.push([key, parsed]);
  }
  return Object.fromEntries(parsedEntries);
}

function parseNumberRecord(value: unknown): Record<string, number> | undefined {
  if (!isRecord(value)) return undefined;
  const entries = Object.entries(value);
  if (entries.length > MAX_CAPABILITY_ITEMS) return undefined;
  const parsedEntries: [string, number][] = [];
  for (const [key, item] of entries) {
    if (FORBIDDEN_DYNAMIC_KEYS.has(key)) return undefined;
    if (!isNonnegativeInteger(item)) return undefined;
    parsedEntries.push([key, item]);
  }
  return Object.fromEntries(parsedEntries);
}

function parseMaximum(value: unknown): number | null | undefined {
  if (value === null) return null;
  return isNonnegativeInteger(value) ? value : undefined;
}

function isNonnegativeInteger(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value) && value >= 0;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function hasOnlyKeys(value: Record<string, unknown>, allowed: string[]): boolean {
  return Object.keys(value).every((key) => allowed.includes(key));
}
