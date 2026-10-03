import { describe, expect, it } from "vitest";

import previewCatalog from "@/features/session/model-catalog.preview.json";

import { publicModelCatalog } from "./model-catalog-test-fixtures";
import {
  parseModelCatalog,
  projectChatModelCatalog,
  projectImageModelCatalog,
  projectVideoModelCatalog,
} from "./model-catalog-contract";

describe("model catalog contract", () => {
  it("keeps documented but unpriced videos informational until pricing is complete", () => {
    const payload = publicModelCatalog();
    const model = payload.items[2];
    model.verification = "pending-verification";
    model.operations[0].enabled = false;
    model.operations[0].video.price_by_option = {};
    expect(parseModelCatalog(payload).items[2].operations[0].enabled).toBe(false);
    expect(projectVideoModelCatalog(parseModelCatalog(payload)).items.some((item) => item.id === model.id)).toBe(false);
    model.operations[0].enabled = true;
    expect(() => parseModelCatalog(payload)).toThrow();
  });
  it("accepts the public unified model catalog with explicit image metadata", () => {
    const payload = publicModelCatalog();

    const catalog = parseModelCatalog(payload);

    expect(catalog.default_model_id).toBe("chatgpt");
    expect(catalog.items[0].operations[0].image).toMatchObject({
      quality_label: "Режим",
      show_output_count: false,
      price_by_variant: { "1K:1:1": 10, "1K:16:9": 12, "2K:1:1": 18, "2K:16:9": 20 },
    });
  });


  it("accepts all-disabled audio operations only while verification is pending", () => {
    const payload = publicModelCatalog();
    payload.default_model_id = "suno_v6";
    payload.items.push({
      id: "suno_v6",
      name: "Suno V6",
      description: "Pending audio candidate",
      kind: "audio",
      categories: ["video-audio"],
      verification: "pending-verification",
      operations: [
        {
          id: "generate",
          kind: "audio",
          enabled: false,
          inputs: {
            images: { support: "unknown", enabled: false },
            video: { support: "unknown", enabled: false },
            audio: { support: "unknown", enabled: false },
            documents: { support: "unknown", enabled: false },
            max_total_bytes: 0,
          },
          audio: { tasks: [], languages: [], voices: [], formats: ["mp3", "m4a", "wav"], max_duration_sec: 360 },
          music: {
            title: "Generate",
            group: "create",
            output_kind: "audio",
            supports_max: true,
            supports_custom_model: false,
            supports_persona: false,
            supports_audio_format: true,
            min_sources: 0,
            max_sources: 0,
            min_uploads: 0,
            max_uploads: 0,
            estimate_credits: 25,
            max_estimate_credits: 40,
            unavailable_reason: "pending verification",
          },
        },
      ],
    });

    expect(parseModelCatalog(payload).items.at(-1)?.verification).toBe("pending-verification");

    payload.items[payload.items.length - 1].verification = "verified-contract";
    expect(() => parseModelCatalog(payload)).toThrow();
  });
  it("accepts sparse image price variants when every offered dimension is represented", () => {
    const payload = publicModelCatalog();
    delete payload.items[0].operations[0].image.price_by_variant["1K:16:9"];

    const catalog = parseModelCatalog(payload);

    expect(catalog.items[0].operations[0].image?.price_by_variant).toEqual({
      "1K:1:1": 10,
      "2K:1:1": 18,
      "2K:16:9": 20,
    });
  });

  it("projects optional image and video prompt character limits", () => {
    const payload = publicModelCatalog();
    payload.items[0].operations[0].image.min_prompt_chars = 3;
    payload.items[0].operations[0].image.max_prompt_chars = 3000;
    payload.items[2].operations[0].video.min_prompt_chars = 3;
    payload.items[2].operations[0].video.max_prompt_chars = 1800;

    const catalog = parseModelCatalog(payload);

    expect(projectImageModelCatalog(catalog).items[0]).toMatchObject({
      min_prompt_chars: 3,
      max_prompt_chars: 3000,
    });
    expect(projectVideoModelCatalog(catalog).items[0]).toMatchObject({
      min_prompt_chars: 3,
      max_prompt_chars: 1800,
    });
  });

  it("projects automatic video resolution without rejecting the strict catalog", () => {
    const payload = publicModelCatalog();
    payload.items[2].operations[0].video.automatic_resolution = true;

    const catalog = parseModelCatalog(payload);

    expect(projectVideoModelCatalog(catalog).items[0]).toMatchObject({
      automatic_resolution: true,
      default_resolution: "720p",
      allowed_resolutions: ["720p"],
    });
  });

  it("parses and projects the generated backend preview fixture", () => {
    const catalog = parseModelCatalog(previewCatalog);
    const images = projectImageModelCatalog(catalog);
    const chat = projectChatModelCatalog(catalog);
    const video = projectVideoModelCatalog(catalog);

    expect(catalog.items.length).toBeGreaterThan(30);
    expect(chat.default_model_id).toBe("chatgpt");
    expect(images.items[0]).toEqual(expect.objectContaining({
      default_aspect_ratio: expect.any(String),
      max_prompt_bytes: expect.any(Number),
      price_by_variant: expect.any(Object),
      quality_label: expect.any(String),
      show_output_count: expect.any(Boolean),
    }));
    expect(video.items[0]?.variants?.[0]).toEqual(expect.objectContaining({ audio: null, fps: null }));
    for (const projection of [images.items, chat.items, video.items]) {
      for (const model of projection) {
        expect(model.capabilities).toEqual(catalog.items.find((entry) => entry.id === model.id)?.capabilities);
        expect(model.capabilities).toBeDefined();
      }
    }
  });

  it("rejects capabilities copied from another model purpose", () => {
    const catalog = structuredClone(previewCatalog);
    const image = catalog.items.find((model) => model.kind === "image")!;
    const text = catalog.items.find((model) => model.kind === "text")!;
    image.capabilities = text.capabilities;
    expect(() => parseModelCatalog(catalog)).toThrow();
  });

  it.each([
    ["duplicate models", () => {
      const payload = publicModelCatalog();
      payload.items.push({ ...payload.items[1] });
      return payload;
    }],
    ["missing default", () => ({ ...publicModelCatalog(), default_model_id: "missing" })],
    ["private model field", () => ({
      ...publicModelCatalog(),
      items: [{ ...publicModelCatalog().items[0], provider_model_id: "private" }],
    })],
    ["invalid operation kind", () => {
      const payload = publicModelCatalog();
      payload.items[0].operations[0].kind = "file";
      return payload;
    }],
    ["missing image quality label", () => {
      const payload = publicModelCatalog();
      delete payload.items[0].operations[0].image.quality_label;
      return payload;
    }],
    ["missing image output-count display flag", () => {
      const payload = publicModelCatalog();
      delete payload.items[0].operations[0].image.show_output_count;
      return payload;
    }],
    ["bad image default", () => {
      const payload = publicModelCatalog();
      payload.items[0].operations[0].image.default_aspect_ratio = "4:3";
      return payload;
    }],
    ["unpriced default image variant", () => {
      const payload = publicModelCatalog();
      delete payload.items[0].operations[0].image.price_by_variant["2K:16:9"];
      return payload;
    }],
    ["unpriced offered image quality", () => {
      const payload = publicModelCatalog();
      delete payload.items[0].operations[0].image.price_by_variant["1K:1:1"];
      delete payload.items[0].operations[0].image.price_by_variant["1K:16:9"];
      return payload;
    }],
    ["unpriced offered image aspect ratio", () => {
      const payload = publicModelCatalog();
      delete payload.items[0].operations[0].image.price_by_variant["1K:1:1"];
      delete payload.items[0].operations[0].image.price_by_variant["2K:1:1"];
      return payload;
    }],
    ["video variant without a price option", () => {
      const payload = publicModelCatalog();
      delete payload.items[2].operations[0].video.price_by_option["720p:8"];
      return payload;
    }],
    ["video price without a matching variant", () => {
      const payload = publicModelCatalog();
      payload.items[2].operations[0].video.price_by_option["1080p:8"] = 200;
      return payload;
    }],
    ["primary model kind without a matching enabled operation", () => {
      const payload = publicModelCatalog();
      payload.items[0].kind = "text";
      return payload;
    }],
  ])("rejects %s", (_name, makePayload) => {
    expect(() => parseModelCatalog(makePayload())).toThrow();
  });
});
