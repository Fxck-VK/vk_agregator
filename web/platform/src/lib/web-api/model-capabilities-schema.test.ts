import { describe, expect, it } from "vitest";

import { modelCapabilitiesSchema } from "./model-capabilities-schema";

const unsupportedInput = { support: "unsupported", extensions: [], max_count: 0 };
const unknownInput = { support: "unknown", extensions: [], max_count: null };
const supportedInput = { support: "supported", extensions: null, max_count: null };

const textProfile = {
  text: {
    images: supportedInput,
    videos: unknownInput,
    files: supportedInput,
  },
  notes: ["API подтверждён отдельно от приложения."],
};

const textApplicationProfile = {
  text: {
    images: unsupportedInput,
    videos: unsupportedInput,
    files: unsupportedInput,
  },
  notes: ["Приложение отправляет только prompt."],
};

const videoProfile = {
  video: {
    images: { support: "supported", extensions: [".jpg", ".png"], max_count: 2 },
    videos: { support: "supported", extensions: [".mp4", ".mov"], max_count: 1 },
    allowed_image_counts: [0, 1, 2],
    duration: {
      mode: "reference_video",
      min_seconds: 3,
      max_seconds: 30,
      allowed_seconds: [5, 10],
      by_resolution: { "720p": [5, 10], "1080p": [5] },
      max_by_orientation: { portrait: 15, landscape: 30 },
    },
    resolutions: ["720p", "1080p"],
    quality_modes: ["std", "pro"],
    aspect_ratios: ["16:9", "9:16"],
    audio: { mode: "preserve_source", selectable: true },
    start_frame: "required",
    end_frame: "optional",
  },
  notes: ["Video route metadata."],
};

describe("modelCapabilitiesSchema", () => {
  it("rejects an API profile for a different model purpose", () => {
    expect(modelCapabilitiesSchema.safeParse({ schema_version: 1, api: videoProfile, application: textApplicationProfile }).success).toBe(false);
  });
  it("accepts backend text capabilities with separate API and application support", () => {
    const parsed = modelCapabilitiesSchema.safeParse({
      schema_version: 1,
      api: textProfile,
      application: textApplicationProfile,
    });

    expect(parsed.success).toBe(true);
  });

  it("accepts backend video capabilities with duration, audio, and frame metadata", () => {
    const parsed = modelCapabilitiesSchema.safeParse({
      schema_version: 1,
      api: videoProfile,
      application: {
        video: {
          ...videoProfile.video,
          images: unsupportedInput,
          videos: unsupportedInput,
          audio: { mode: "silent", selectable: false },
          start_frame: "unsupported",
          end_frame: "unsupported",
        },
      },
    });

    expect(parsed.success).toBe(true);
  });

  it("rejects private provider fields from capability profiles", () => {
    const parsed = modelCapabilitiesSchema.safeParse({
      schema_version: 1,
      api: {
        ...textProfile,
        provider_model_id: "gpt-6-astra",
      },
      application: textApplicationProfile,
    });

    expect(parsed.success).toBe(false);
  });

  it("rejects profiles that mix multiple model purposes", () => {
    const parsed = modelCapabilitiesSchema.safeParse({
      schema_version: 1,
      api: {
        ...textProfile,
        video: videoProfile.video,
      },
      application: textApplicationProfile,
    });

    expect(parsed.success).toBe(false);
  });
});
