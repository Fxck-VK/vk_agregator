import { describe, expect, it } from "vitest";

import { parseModelCatalog } from "@/features/models/model-catalog-contract";
import { publicInputs, publicModelCatalog } from "@/features/models/model-catalog-test-fixtures";

import { projectMusicModelCatalog } from "./music-model-catalog";

function pendingMusicCatalog() {
  const payload = publicModelCatalog();
  payload.default_model_id = "suno_v6";
  payload.items.push({
    id: "suno_v6",
    name: "Suno V6",
    description: "Pending Suno candidate",
    kind: "audio",
    categories: ["video-audio"],
    verification: "pending-verification",
    operations: [
      {
        id: "generate",
        kind: "audio",
        enabled: false,
        inputs: publicInputs(),
        audio: {
          formats: ["mp3", "m4a", "wav"],
          languages: [],
          max_duration_sec: 360,
          tasks: [],
          voices: [],
        },
        music: {
          estimate_credits: 25,
          group: "create",
          max_estimate_credits: 40,
          max_sources: 0,
          max_uploads: 0,
          min_sources: 0,
          min_uploads: 0,
          output_kind: "audio",
          supports_audio_format: true,
          supports_custom_model: false,
          supports_max: true,
          supports_persona: false,
          title: "Generate",
          unavailable_reason: "pending verification",
        },
      },
      {
        id: "mashup",
        kind: "audio",
        enabled: false,
        inputs: publicInputs(),
        audio: {
          formats: ["mp3", "m4a", "wav"],
          max_duration_sec: 360,
        },
        music: {
          estimate_credits: 30,
          group: "edit",
          max_sources: 2,
          max_uploads: 0,
          min_sources: 2,
          min_uploads: 0,
          output_kind: "audio",
          supports_audio_format: false,
          supports_custom_model: false,
          supports_max: false,
          supports_persona: false,
          title: "Mashup",
        },
      },
    ],
  });
  return payload;
}

describe("music model catalog projection", () => {
  it("projects pending disabled audio candidates without making them look runnable", () => {
    const catalog = parseModelCatalog(pendingMusicCatalog());

    const music = projectMusicModelCatalog(catalog);

    expect(music.defaultModelId).toBe("suno_v6");
    expect(music.models).toHaveLength(1);
    expect(music.models[0]).toMatchObject({
      availability: "unverified",
      enabled: false,
      id: "suno_v6",
      statusReason: "pending verification",
    });
    expect(music.models[0].operations).toEqual(expect.arrayContaining([
      expect.objectContaining({
        enabled: false,
        estimateCredits: 25,
        id: "generate",
        maxEstimateCredits: 40,
        supportsAudioFormat: true,
        supportsMaxMode: true,
      }),
      expect.objectContaining({
        enabled: false,
        id: "mashup",
        requirement: expect.objectContaining({ maxTracks: 2, minTracks: 2, ownedUpload: false }),
      }),
    ]));
  });

  it("ignores unknown music IDs instead of surfacing unsupported operations", () => {
    const payload = pendingMusicCatalog();
    payload.items[payload.items.length - 1].id = "provider_native_suno";
    payload.default_model_id = "chatgpt";

    const music = projectMusicModelCatalog(parseModelCatalog(payload));

    expect(music.models).toEqual([]);
  });

  it("preserves create_model multi-upload limits from the server catalog", () => {
    const payload = publicModelCatalog();
    payload.default_model_id = "suno_v6";
    payload.items.push({
      id: "suno_v6",
      name: "Suno V6",
      description: "Verified Suno model",
      kind: "audio",
      categories: ["video-audio"],
      verification: "verified-contract",
      operations: [
        {
          id: "create_model",
          kind: "audio",
          enabled: true,
          inputs: publicInputs(),
          audio: {
            formats: ["mp3", "m4a", "wav"],
            max_duration_sec: 360,
          },
          music: {
            estimate_credits: 100,
            group: "create",
            max_sources: 0,
            max_uploads: 24,
            min_sources: 0,
            min_uploads: 6,
            output_kind: "model",
            supports_audio_format: false,
            supports_custom_model: false,
            supports_max: false,
            supports_persona: false,
            title: "Create model",
          },
        },
      ],
    });

    const music = projectMusicModelCatalog(parseModelCatalog(payload));

    expect(music.models[0]).toMatchObject({
      availability: "available",
      enabled: true,
    });
    expect(music.models[0].operations[0]).toMatchObject({
      details: expect.arrayContaining(["Результат: модель", "Загружаемые аудио: 6–24"]),
      enabled: true,
      id: "create_model",
      requirement: expect.objectContaining({
        maxUploads: 24,
        minUploads: 6,
        ownedUpload: true,
      }),
      supportsUploads: true,
    });
  });
});
