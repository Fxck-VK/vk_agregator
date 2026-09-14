import { describe, expect, test } from "vitest";

import { parseModelCapabilities } from "./model-capabilities";

const unsupportedInput = { support: "unsupported", extensions: [], max_count: 0 };
const unknownInput = { support: "unknown", extensions: [], max_count: null };
const supportedInput = { support: "supported", extensions: [".png"], max_count: 16 };

const textCapabilities = {
  schema_version: 1,
  api: {
    text: {
      images: supportedInput,
      videos: unknownInput,
      files: { support: "supported", extensions: null, max_count: null },
    },
    notes: ["API проверен отдельно."],
  },
  application: {
    text: {
      images: unsupportedInput,
      videos: unsupportedInput,
      files: unsupportedInput,
    },
  },
};

const videoCapabilities = {
  schema_version: 1,
  api: {
    video: {
      images: supportedInput,
      videos: { support: "supported", extensions: [".mp4", ".mov"], max_count: 1 },
      allowed_image_counts: [0, 1, 2],
      duration: {
        mode: "reference_video",
        min_seconds: 3,
        max_seconds: 30,
        allowed_seconds: [5, 10],
        by_resolution: { "720p": [5, 10] },
        max_by_orientation: { portrait: 15 },
      },
      resolutions: ["720p"],
      quality_modes: ["std"],
      aspect_ratios: ["16:9"],
      audio: { mode: "preserve_source", selectable: true },
      start_frame: "required",
      end_frame: "optional",
    },
  },
  application: {
    video: {
      images: unsupportedInput,
      videos: unsupportedInput,
      allowed_image_counts: [0],
      duration: {
        mode: "selected",
        min_seconds: 5,
        max_seconds: 5,
        allowed_seconds: [5],
      },
      resolutions: ["720p"],
      quality_modes: ["std"],
      aspect_ratios: ["16:9"],
      audio: { mode: "silent", selectable: false },
      start_frame: "unsupported",
      end_frame: "unsupported",
    },
  },
};

describe("parseModelCapabilities", () => {
  test("accepts text capabilities with unknown native inputs kept distinct", () => {
    const parsed = parseModelCapabilities(textCapabilities);

    expect(parsed?.api.text?.videos.support).toBe("unknown");
    expect(parsed?.application.text?.videos.support).toBe("unsupported");
  });

  test("accepts video duration, audio, frame, and typed array metadata", () => {
    const parsed = parseModelCapabilities(videoCapabilities);

    expect(parsed?.api.video?.duration.by_resolution?.["720p"]).toEqual([5, 10]);
    expect(parsed?.api.video?.duration.max_by_orientation?.portrait).toBe(15);
    expect(parsed?.api.video?.audio.mode).toBe("preserve_source");
    expect(parsed?.api.video?.end_frame).toBe("optional");
  });

  test("rejects unsupported schema versions and input statuses", () => {
    expect(parseModelCapabilities({ ...textCapabilities, schema_version: 2 })).toBeUndefined();
    expect(
      parseModelCapabilities({
        ...textCapabilities,
        api: { text: { ...textCapabilities.api.text, images: { ...supportedInput, support: "maybe" } } },
      }),
    ).toBeUndefined();
  });

  test("rejects private fields, malformed arrays, and malformed counts", () => {
    expect(parseModelCapabilities({ ...textCapabilities, provider_model_id: "private" })).toBeUndefined();
    expect(
      parseModelCapabilities({
        ...textCapabilities,
        api: { text: { ...textCapabilities.api.text, images: { ...supportedInput, extensions: [".png", 7] } } },
      }),
    ).toBeUndefined();
    expect(
      parseModelCapabilities({
        ...textCapabilities,
        api: { text: { ...textCapabilities.api.text, images: { ...supportedInput, max_count: -1 } } },
      }),
    ).toBeUndefined();
    expect(
      parseModelCapabilities({
        ...textCapabilities,
        api: { text: { ...textCapabilities.api.text, images: { ...supportedInput, extensions: Array.from({ length: 121 }, (_, index) => `.${index}`) } } },
      }),
    ).toBeUndefined();
  });

  test("rejects multi-purpose profiles and mismatched api/application purpose", () => {
    expect(parseModelCapabilities({ ...textCapabilities, api: { ...textCapabilities.api, image: {} } })).toBeUndefined();
    expect(parseModelCapabilities({ ...textCapabilities, application: videoCapabilities.application })).toBeUndefined();
  });

  test("rejects unsafe or oversized dynamic duration maps", () => {
    const unsafeResolutionMap = Object.fromEntries([["__proto__", [5]]]);
    const oversizedOrientationMap = Object.fromEntries(
      Array.from({ length: 121 }, (_, index) => [`orientation-${index}`, index]),
    );

    expect(
      parseModelCapabilities({
        ...videoCapabilities,
        api: {
          video: {
            ...videoCapabilities.api.video,
            duration: { ...videoCapabilities.api.video.duration, by_resolution: unsafeResolutionMap },
          },
        },
      }),
    ).toBeUndefined();
    expect(
      parseModelCapabilities({
        ...videoCapabilities,
        api: {
          video: {
            ...videoCapabilities.api.video,
            duration: { ...videoCapabilities.api.video.duration, max_by_orientation: oversizedOrientationMap },
          },
        },
      }),
    ).toBeUndefined();
  });
});
