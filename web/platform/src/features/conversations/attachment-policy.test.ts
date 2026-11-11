import { describe, expect, it } from "vitest";
import catalog from "@/features/session/model-catalog.preview.json";
import { projectGenerationModelCatalog, type GenerationModel } from "@/features/models/generation-model-catalog";
import { parseModelCatalog, type PublicOperation } from "@/features/models/model-catalog-contract";
import { attachmentPolicy, acceptsAttachment } from "./attachment-policy";

const models = projectGenerationModelCatalog(parseModelCatalog(catalog)).items;
const image = models.find(model => model.id === "nano_banana_2")! as GenerationModel;
const operation = (image.operations as PublicOperation[])[0];
const withInput = (input: Partial<PublicOperation["inputs"]["images"]>): GenerationModel => ({
  ...image, operations: [{ ...operation, inputs: { ...operation.inputs, images: { ...operation.inputs.images, ...input } } }],
});

describe("attachment policy", () => {
  it("uses admitted operation inputs for every generation model, not provider capability claims", () => {
    for (const model of models.filter((item): item is GenerationModel => item.category !== "audio")) {
      const op = (model.operations as PublicOperation[]).find(item => item.enabled && item.kind === (model.category === "images" ? "image" : model.category));
      const policy = attachmentPolicy(model);
      expect(policy.enabled, model.id).toBe(op?.inputs.images.enabled === true);
      if (policy.enabled) {
        expect(policy.maxCount, model.id).toBe(op!.inputs.images.max_count);
        expect(policy.maxBytes, model.id).toBe(op!.inputs.images.max_bytes);
        expect(policy.accept, model.id).not.toMatch(/\*|video|audio|pdf/);
      }
    }
    expect(attachmentPolicy(models.find(model => model.id === "video_veo_3_1_quality") as GenerationModel).enabled).toBe(false);
  });

  it("fails closed for missing, disabled, unknown or incomplete input contracts", () => {
    expect(attachmentPolicy().enabled).toBe(false);
    for (const input of [{ enabled: false }, { support: "unknown" as const }, { formats: [] }, { max_bytes: 0 }, { max_count: 0 }]) {
      expect(attachmentPolicy(withInput(input)).enabled).toBe(false);
    }
    expect(attachmentPolicy({ ...image, operations: [] }).enabled).toBe(false);
    expect(attachmentPolicy({ ...image, operations: [{ ...operation, enabled: false }] }).enabled).toBe(false);
  });

  it("projects formats, allowed counts and required status of the same operation", () => {
    const policy = attachmentPolicy(withInput({ required: true, allowed_counts: [2, 4], max_count: 4, formats: [{ mime: "image/png", extension: ".png" }] }));
    expect(policy.accept).toBe("image/png,.png");
    expect(policy.allowedCounts).toEqual([2, 4]);
    expect(policy.required).toBe(true);
    expect(acceptsAttachment(policy, { mimeType: "image/jpeg", name: "photo.jpg" })).toBe(false);
    expect(acceptsAttachment(policy, { mimeType: "image/png", name: "photo.png" })).toBe(true);
    expect(acceptsAttachment(policy, { mimeType: "application/octet-stream", name: "photo.png", file: new File(["x"], "photo.png") })).toBe(true);
    expect(acceptsAttachment(policy, { mimeType: "application/pdf", name: "photo.png", file: new File(["x"], "photo.png", { type: "application/pdf" }) })).toBe(false);
  });
});
