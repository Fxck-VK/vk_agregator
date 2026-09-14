import { describe, expect, it } from "vitest";

import {
  getFileActionModels,
  type WorkspaceModelCatalog,
  type WorkspaceOperation,
} from "./file-action-models";

const imageInputsEnabled: WorkspaceOperation["inputs"] = {
  audio: { enabled: false, support: "unsupported" },
  documents: { enabled: false, support: "unsupported" },
  images: { enabled: true, support: "supported" },
  max_total_bytes: 10_000_000,
  video: { enabled: false, support: "unsupported" },
};

const imageInputsDisabled: WorkspaceOperation["inputs"] = {
  ...imageInputsEnabled,
  images: { enabled: false, support: "supported" },
};

function imageOperation(
  id: string,
  inputs: WorkspaceOperation["inputs"] = imageInputsEnabled,
): WorkspaceOperation {
  return {
    enabled: true,
    id,
    image: {
      allowed_aspect_ratios: ["1:1", "16:9"],
      default_aspect_ratio: "1:1",
      default_quality: "2K",
      max_output_count: 1,
      max_reference_images: 1,
      price_by_quality: { "2K": 55 },
      price_by_variant: { "2K:1:1": 55 },
      quality_label: "Разрешение",
      quality_options: ["2K"],
      show_output_count: true,
      supports_reference_image: true,
    },
    inputs,
    kind: "image",
  };
}

const catalog: WorkspaceModelCatalog = {
  default_model_id: "chatgpt",
  items: [
    {
      categories: ["popular", "images"],
      description: "Plain text-to-image generation.",
      id: "image-generator",
      kind: "image",
      name: "Image Generator",
      operations: [imageOperation("generate")],
      verification: "verified-contract",
    },
    {
      categories: ["popular", "images"],
      description: "Documented but not wired for web image inputs.",
      id: "explicit-file-tools-without-inputs",
      kind: "image",
      name: "Disabled File Tools",
      operations: [
        imageOperation("edit", imageInputsDisabled),
        imageOperation("enhance", imageInputsDisabled),
        imageOperation("remove-background", imageInputsDisabled),
      ],
      verification: "verified-contract",
    },
    {
      categories: ["popular", "video-audio"],
      description: "Can generate video from uploaded images.",
      id: "video-with-start-image",
      kind: "video",
      name: "Video With Start Image",
      operations: [{
        enabled: true,
        id: "generate",
        inputs: imageInputsEnabled,
        kind: "video",
        video: {
          allowed_aspect_ratios: ["16:9"],
          allowed_durations_sec: [5],
          allowed_resolutions: ["720p"],
          default_aspect_ratio: "16:9",
          default_duration_sec: 5,
          default_resolution: "720p",
          end_image: "unsupported",
          price_by_option: { "720p:5": 25 },
          start_image: "required",
          variants: [{
            aspect_ratio: "16:9",
            audio: false,
            duration_sec: 5,
            fps: 24,
            resolution: "720p",
          }],
        },
      }],
      verification: "verified-contract",
    },
    {
      categories: ["popular", "images"],
      description: "Explicit file editing operations.",
      id: "real-file-tools",
      kind: "image",
      name: "Real File Tools",
      operations: [
        imageOperation("edit"),
        imageOperation("enhance"),
        imageOperation("remove-background"),
      ],
      verification: "verified-contract",
    },
  ],
  schema_version: 1,
};

describe("file action model catalog", () => {
  it("does not invent file actions from plain image generation or disabled image inputs", () => {
    expect(getFileActionModels("edit", {
      ...catalog,
      items: catalog.items.slice(0, 2),
    })).toEqual([]);
    expect(getFileActionModels("enhance", {
      ...catalog,
      items: catalog.items.slice(0, 2),
    })).toEqual([]);
    expect(getFileActionModels("remove-background", {
      ...catalog,
      items: catalog.items.slice(0, 2),
    })).toEqual([]);
    expect(getFileActionModels("animate", {
      ...catalog,
      items: [{
        ...catalog.items[2]!,
        operations: [{
          ...catalog.items[2]!.operations[0]!,
          inputs: imageInputsDisabled,
        }],
      }],
    })).toEqual([]);
  });

  it("keeps only enabled task-specific operations with image inputs and catalog prices", () => {
    expect(getFileActionModels("animate", catalog).map((model) => [model.id, model.cost])).toEqual([
      ["video-with-start-image", 25],
    ]);
    expect(getFileActionModels("edit", catalog).map((model) => [model.id, model.cost])).toEqual([
      ["real-file-tools", 55],
    ]);
    expect(getFileActionModels("enhance", catalog).map((model) => [model.id, model.cost])).toEqual([
      ["real-file-tools", 55],
    ]);
    expect(getFileActionModels("remove-background", catalog).map((model) => [model.id, model.cost])).toEqual([
      ["real-file-tools", 55],
    ]);
  });
});
