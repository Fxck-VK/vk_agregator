
import type { PublicCatalog, PublicOperation } from "./model-catalog-contract";

type PublicImageControlsFixture =
  & Omit<NonNullable<PublicOperation["image"]>, "quality_label" | "show_output_count">
  & Partial<Pick<NonNullable<PublicOperation["image"]>, "quality_label" | "show_output_count">>
  & Record<string, unknown>;

type PublicOperationBaseFixture =
  & Omit<PublicOperation, "kind" | "text" | "image" | "video">
  & {
    kind: string;
  }
  & Record<string, unknown>;

type PublicTextOperationFixture = PublicOperationBaseFixture & {
  text: NonNullable<PublicOperation["text"]>;
};

type PublicImageOperationFixture = PublicOperationBaseFixture & {
  image: PublicImageControlsFixture;
};

type PublicVideoOperationFixture = PublicOperationBaseFixture & {
  video: NonNullable<PublicOperation["video"]> & Record<string, unknown>;
};

type PublicCatalogModelBaseFixture =
  & Omit<PublicCatalog["items"][number], "kind" | "operations">
  & {
    kind: string;
  }
  & Record<string, unknown>;

type PublicCatalogModelFixture = PublicCatalogModelBaseFixture & {
  operations: Array<PublicTextOperationFixture | PublicImageOperationFixture | PublicVideoOperationFixture>;
};

type PublicImageCatalogModelFixture = PublicCatalogModelBaseFixture & {
  operations: [PublicImageOperationFixture];
};

type PublicTextCatalogModelFixture = PublicCatalogModelBaseFixture & {
  operations: [PublicTextOperationFixture];
};

type PublicVideoCatalogModelFixture = PublicCatalogModelBaseFixture & {
  operations: [PublicVideoOperationFixture];
};

type PublicCatalogFixture = Omit<PublicCatalog, "items"> & {
  items: [
    PublicImageCatalogModelFixture,
    PublicTextCatalogModelFixture,
    PublicVideoCatalogModelFixture,
    ...PublicCatalogModelFixture[],
  ];
};

export function publicInputs(): PublicOperation["inputs"] {
  return {
    images: { support: "unknown", enabled: false },
    video: { support: "unknown", enabled: false },
    audio: { support: "unknown", enabled: false },
    documents: { support: "unknown", enabled: false },
    max_total_bytes: 0,
  };
}

export function publicModelCatalog(): PublicCatalogFixture {
  return {
    schema_version: 1,
    default_model_id: "chatgpt",
    items: [
      {
        id: "image",
        name: "Image",
        description: "Server image description",
        kind: "image",
        categories: ["popular", "images"],
        verification: "verified-contract",
        version: "2026-09-14",
        operations: [
          {
            id: "generate",
            kind: "image",
            enabled: true,
            inputs: publicInputs(),
            image: {
              quality_options: ["1K", "2K"],
              default_quality: "2K",
              allowed_aspect_ratios: ["1:1", "16:9"],
              default_aspect_ratio: "16:9",
              max_output_count: 4,
              supports_reference_image: true,
              max_reference_images: 2,
              price_by_quality: { "1K": 10, "2K": 20 },
              price_by_variant: { "1K:1:1": 10, "1K:16:9": 12, "2K:1:1": 18, "2K:16:9": 20 },
              quality_label: "Режим",
              show_output_count: false,
              max_prompt_bytes: 2048,
            },
          },
        ],
      },
      {
        id: "chatgpt",
        name: "Chat",
        description: "Server chat description",
        kind: "text",
        categories: ["popular", "text", "free", "study-work"],
        verification: "legacy-unverified",
        operations: [
          {
            id: "reply",
            kind: "text",
            enabled: true,
            inputs: publicInputs(),
            text: {
              estimate_credits: 0,
              max_prompt_bytes: 4096,
              max_output_tokens: 1024,
              context_tokens: 8192,
            },
          },
        ],
      },
      {
        id: "video",
        name: "Video",
        description: "Server video description",
        kind: "video",
        categories: ["popular", "video-audio"],
        verification: "legacy-unverified",
        operations: [
          {
            id: "generate",
            kind: "video",
            enabled: true,
            inputs: publicInputs(),
            video: {
              allowed_resolutions: ["720p"],
              allowed_durations_sec: [8],
              allowed_aspect_ratios: ["16:9"],
              default_resolution: "720p",
              default_duration_sec: 8,
              default_aspect_ratio: "16:9",
              price_by_option: { "720p:8": 100 },
              variants: [
                {
                  resolution: "720p",
                  duration_sec: 8,
                  aspect_ratio: "16:9",
                  fps: 24,
                  audio: false,
                },
              ],
              start_image: "unsupported",
              end_image: "unsupported",
            },
          },
        ],
      },
    ],
  };
}
