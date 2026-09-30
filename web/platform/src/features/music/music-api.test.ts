import { describe, expect, it } from "vitest";

import type { MusicWorkspaceActionRequest, MusicWorkspaceOperation, MusicTrack } from "./MusicWorkspace";
import { buildMusicPrepareBody, musicSummaryFromResult, normalizeMusicArtifactURL } from "./music-api";

const operation: MusicWorkspaceOperation = {
  enabled: true,
  id: "mashup",
  requirement: { maxTracks: 2, minTracks: 2 },
  supportsAudioFormat: true,
  supportsMaxMode: true,
};

const request: MusicWorkspaceActionRequest = {
  draft: {
    actionParameters: { mashup: { prompt: "blend hooks" } },
    descriptionPrompt: "make a mashup",
    instrumental: false,
    lyrics: "",
    maxMode: true,
    outputFormat: "wav",
    promptWeight: 0.7,
    style: "hyperpop",
    styleWeight: 0.6,
    targetDurationMode: "custom",
    targetDurationSec: 185,
    title: "Night ride",
    variety: 0.4,
  },
  mode: "description",
  modelId: "suno_v6",
  operationId: "mashup",
  parameters: { prompt: "blend hooks" },
  sourceJobIds: ["ignored-a", "ignored-b"],
  sourceTrackIds: ["track-a", "track-b"],
};

const tracks: readonly MusicTrack[] = [
  {
    artifactPlaybackUrl: "/web/v1/music-artifacts/11111111-1111-4111-8111-111111111111",
    id: "track-a",
    originalIndex: 0,
    sourceJobId: "11111111-1111-4111-8111-aaaaaaaaaaaa",
  },
  {
    artifactPlaybackUrl: "/web/v1/music-artifacts/22222222-2222-4222-8222-222222222222",
    id: "track-b",
    originalIndex: 5,
    sourceJobId: "22222222-2222-4222-8222-bbbbbbbbbbbb",
  },
];

describe("music API helpers", () => {
  it("builds provider-neutral prepare bodies without trusting callback source jobs", () => {
    expect(buildMusicPrepareBody(request, tracks, operation)).toEqual({
      audio_artifact_ids: [],
      model_id: "suno_v6",
      music: {
        action: "mashup",
        audio_format: "wav",
        audio_weight: 0.7,
        custom: true,
        description: "blend hooks",
        duration_sec: 185,
        gpt_description: "blend hooks",
        instrumental: false,
        max_mode: true,
        prompt: "blend hooks",
        style: "hyperpop",
        style_weight: 0.6,
        styles: "hyperpop",
        tags: "hyperpop",
        title: "Night ride",
        variety: "high",
        weirdness: 0.4,
      },
      sources: [
        { audio_index: 1, job_id: "11111111-1111-4111-8111-aaaaaaaaaaaa" },
        { audio_index: 6, job_id: "22222222-2222-4222-8222-bbbbbbbbbbbb" },
      ],
    });
  });

  it("includes owned upload artifacts only for upload-backed requests", () => {
    const uploadRequest: MusicWorkspaceActionRequest = {
      ...request,
      draft: {
        ...request.draft,
        ownedUpload: {
          durationSec: 42,
          id: "33333333-3333-4333-8333-333333333333",
          label: "hook.wav",
        },
      },
      mode: "upload",
      operationId: "upload_extend",
      parameters: { seekSec: 12 },
      sourceJobIds: [],
      sourceTrackIds: [],
    };

    expect(buildMusicPrepareBody(uploadRequest, tracks, {
      enabled: true,
      id: "upload_extend",
      requirement: { ownedUpload: true },
      supportsUploads: true,
    }).audio_artifact_ids).toEqual(["33333333-3333-4333-8333-333333333333"]);
  });

  it("keeps reusable persona and custom model job IDs at the prepare top level", () => {
    const personaBody = buildMusicPrepareBody({
      ...request,
      draft: {
        ...request.draft,
        customModelJobId: "44444444-4444-4444-8444-444444444444",
        personaJobId: "33333333-3333-4333-8333-333333333333",
      },
      mode: "description",
      operationId: "generate",
      parameters: {},
      sourceTrackIds: [],
    }, tracks, {
      enabled: true,
      id: "generate",
      supportsCustomModel: true,
      supportsPersona: true,
    });

    expect(personaBody.persona_job_id).toBe("33333333-3333-4333-8333-333333333333");
    expect(personaBody.custom_model_job_id).toBeUndefined();
    expect(personaBody.music).toEqual(expect.objectContaining({
      action: "generate",
      custom: true,
    }));
    expect(personaBody.music).not.toHaveProperty("persona_job_id");
    expect(personaBody.music).not.toHaveProperty("custom_model_job_id");

    const customModelBody = buildMusicPrepareBody({
      ...request,
      draft: {
        ...request.draft,
        customModelJobId: "44444444-4444-4444-8444-444444444444",
        personaJobId: null,
      },
      operationId: "generate",
      parameters: {},
      sourceTrackIds: [],
    }, tracks, {
      enabled: true,
      id: "generate",
      supportsCustomModel: true,
      supportsPersona: true,
    });

    expect(customModelBody.custom_model_job_id).toBe("44444444-4444-4444-8444-444444444444");
    expect(customModelBody.persona_job_id).toBeUndefined();
    expect(customModelBody.music).not.toHaveProperty("custom_model_job_id");
  });

  it("uses own lyrics as the custom prompt instead of the description", () => {
    const body = buildMusicPrepareBody({
      ...request,
      draft: {
        ...request.draft,
        descriptionPrompt: "describe the arrangement only",
        lyrics: "verse one\nchorus",
      },
      mode: "own_lyrics",
      operationId: "generate",
      parameters: {},
      sourceTrackIds: [],
    }, tracks, {
      enabled: true,
      id: "generate",
      supportsPersona: true,
    });

    expect(body.music).toEqual(expect.objectContaining({
      action: "generate",
      auto_lyrics: false,
      custom: true,
      lyrics: "verse one\nchorus",
      prompt: "verse one\nchorus",
    }));
  });

  it("does not fall back to the description when own lyrics are empty", () => {
    const body = buildMusicPrepareBody({
      ...request,
      draft: {
        ...request.draft,
        descriptionPrompt: "do not use this as lyrics",
        lyrics: "   ",
      },
      mode: "own_lyrics",
      operationId: "generate",
      parameters: {},
      sourceTrackIds: [],
    }, tracks, {
      enabled: true,
      id: "generate",
    });

    expect(body.music).not.toHaveProperty("lyrics");
    expect(body.music).not.toHaveProperty("prompt");
  });

  it.each(["extend", "upload_extend", "inspo"] as const)("keeps own lyrics in the implicit custom prompt for %s", (operationId) => {
    const body = buildMusicPrepareBody({
      ...request,
      draft: {
        ...request.draft,
        descriptionPrompt: "arrangement description",
        lyrics: "continuation verse\nchorus",
      },
      mode: "own_lyrics",
      operationId,
      parameters: {},
    }, tracks, { enabled: true, id: operationId });

    expect(body.music).toEqual(expect.objectContaining({
      lyrics: "continuation verse\nchorus",
      prompt: "continuation verse\nchorus",
    }));
    expect(body.music).not.toHaveProperty("custom");
  });

  it.each(["fade_in", "fade_out"] as const)("uses the fade length instead of the song duration for %s", (operationId) => {
    const body = buildMusicPrepareBody({
      ...request,
      operationId,
      parameters: { fadeSeconds: 7 },
      sourceTrackIds: ["track-a"],
    }, tracks, { enabled: true, id: operationId });

    expect(body.music.duration_sec).toBe(7);
  });

  it("keeps own lyrics out of non-lyric helper prompts", () => {
    const body = buildMusicPrepareBody({
      ...request,
      draft: {
        ...request.draft,
        lyrics: "verse should not become sound prompt",
      },
      mode: "own_lyrics",
      operationId: "sounds",
      parameters: { bpm: 120, musicalKey: "Am", prompt: "short vinyl drum loop", soundType: "loop" },
      sourceTrackIds: [],
    }, tracks, {
      enabled: true,
      id: "sounds",
      requirement: { maxTracks: 0, maxUploads: 0, minTracks: 0, minUploads: 0 },
      supportsAudioFormat: true,
    });

    expect(body.music).toEqual(expect.objectContaining({
      action: "sounds",
      bpm: 120,
      key: "Am",
      prompt: "short vinyl drum loop",
      sound_type: "loop",
    }));
    expect(body.music).not.toHaveProperty("auto_lyrics");
    expect(body.music).not.toHaveProperty("custom");
    expect(body.music).not.toHaveProperty("lyrics");
  });

  it("drops retained tracks and uploads for zero-input helper actions", () => {
    const body = buildMusicPrepareBody({
      ...request,
      draft: {
        ...request.draft,
        ownedUploads: [{
          durationSec: 42,
          id: "33333333-3333-4333-8333-333333333333",
          label: "stale.wav",
        }],
      },
      mode: "upload",
      operationId: "lyrics",
      parameters: { prompt: "write lyrics for a sunrise pop song" },
      sourceTrackIds: ["track-a"],
    }, tracks, {
      enabled: true,
      id: "lyrics",
      requirement: { maxTracks: 0, maxUploads: 0, minTracks: 0, minUploads: 0 },
    });

    expect(body.audio_artifact_ids).toEqual([]);
    expect(body.sources).toEqual([]);
    expect(body.music).toEqual(expect.objectContaining({
      action: "lyrics",
      prompt: "write lyrics for a sunrise pop song",
    }));
  });

  it("omits custom-only generation controls in description mode", () => {
    const body = buildMusicPrepareBody({
      ...request,
      draft: {
        ...request.draft,
        descriptionPrompt: "write an upbeat pop idea",
        maxMode: false,
        targetDurationMode: "custom",
        targetDurationSec: 180,
      },
      mode: "description",
      operationId: "generate",
      parameters: {},
      sourceTrackIds: [],
    }, tracks, {
      enabled: true,
      id: "generate",
      supportsMaxMode: true,
    });

    expect(body.music).toEqual(expect.objectContaining({
      action: "generate",
      instrumental: false,
      prompt: "write an upbeat pop idea",
    }));
    expect(body.music).not.toHaveProperty("duration_sec");
    expect(body.music).not.toHaveProperty("style");
    expect(body.music).not.toHaveProperty("style_weight");
    expect(body.music).not.toHaveProperty("tags");
    expect(body.music).not.toHaveProperty("title");
  });

  it("omits native custom-mode fields from upload_extend while preserving its upload", () => {
    const body = buildMusicPrepareBody({
      ...request,
      draft: {
        ...request.draft,
        instrumental: true,
        maxMode: true,
        ownedUploads: [{
          durationSec: 42,
          id: "33333333-3333-4333-8333-333333333333",
          label: "hook.wav",
        }],
      },
      mode: "upload",
      operationId: "upload_extend",
      parameters: { description: "continue it", seekSec: 12 },
      sourceTrackIds: [],
    }, tracks, {
      enabled: true,
      id: "upload_extend",
      requirement: { maxUploads: 1, minUploads: 1, ownedUpload: true },
      supportsMaxMode: true,
      supportsUploads: true,
    });

    expect(body.audio_artifact_ids).toEqual(["33333333-3333-4333-8333-333333333333"]);
    expect(body.music).toEqual(expect.objectContaining({
      action: "upload_extend",
      continue_at_sec: 12,
      max_mode: true,
      prompt: "continue it",
    }));
    expect(body.music).not.toHaveProperty("custom");
    expect(body.music).not.toHaveProperty("instrumental");
    expect(body.music).not.toHaveProperty("gpt_description");
  });

  it("keeps all allowed upload artifacts for multi-upload operations", () => {
    const ownedUploads = Array.from({ length: 6 }, (_, index) => ({
      durationSec: 30 + index,
      id: `${index + 1}${index + 1}${index + 1}${index + 1}${index + 1}${index + 1}${index + 1}${index + 1}-${index + 1}${index + 1}${index + 1}${index + 1}-4${index + 1}${index + 1}${index + 1}-8${index + 1}${index + 1}${index + 1}-${index + 1}${index + 1}${index + 1}${index + 1}${index + 1}${index + 1}${index + 1}${index + 1}${index + 1}${index + 1}${index + 1}${index + 1}`,
      label: `take-${index + 1}.wav`,
    }));

    const body = buildMusicPrepareBody({
      ...request,
      draft: {
        ...request.draft,
        ownedUploads,
      },
      mode: "upload",
      operationId: "create_model",
      parameters: { name: "Lead voice" },
      sourceTrackIds: [],
    }, tracks, {
      enabled: true,
      id: "create_model",
      requirement: { maxUploads: 24, minUploads: 6, ownedUpload: true },
      supportsUploads: true,
    });

    expect(body.audio_artifact_ids).toEqual(ownedUploads.map((upload) => upload.id));
  });

  it("accepts only frontend proxied music artifact URLs", () => {
    expect(normalizeMusicArtifactURL("/web/v1/music-artifacts/11111111-1111-4111-8111-111111111111")).toBe(
      "/web/v1/music-artifacts/11111111-1111-4111-8111-111111111111",
    );
    expect(normalizeMusicArtifactURL("/api/web/v1/music-artifacts/11111111-1111-4111-8111-111111111111")).toBeNull();
    expect(normalizeMusicArtifactURL("https://cdn.example.invalid/audio.mp3")).toBeNull();
  });

  it("summarizes text results and keeps only safe artifact downloads", () => {
    expect(musicSummaryFromResult({
      artifacts: [
        {
          format: "txt",
          id: "33333333-3333-4333-8333-333333333333",
          kind: "lyrics",
          url: "/web/v1/music-artifacts/33333333-3333-4333-8333-333333333333",
        },
        {
          id: "44444444-4444-4444-8444-444444444444",
          kind: "audio",
          url: "https://cdn.example.invalid/private.mp3",
        },
      ],
      bpm: { average: 126, maximum: 130, minimum: 122 },
      job_id: "55555555-5555-4555-8555-555555555555",
      lyrics: [{ tags: "verse", text: "line one\nline two", title: "Draft lyric" }],
      tags: "dream pop",
      tracks: [],
    })).toEqual({
      artifacts: [{
        format: "txt",
        kind: "lyrics",
        label: "Скачать файл TXT",
        url: "/web/v1/music-artifacts/33333333-3333-4333-8333-333333333333",
      }],
      bpm: { average: 126, maximum: 130, minimum: 122 },
      id: "55555555-5555-4555-8555-555555555555",
      lyrics: [{ tags: "verse", text: "line one\nline two", title: "Draft lyric" }],
      model: null,
      persona: null,
      tags: "dream pop",
      voice: null,
    });
  });

  it("maps typed action parameters for edit/export tools", () => {
    const replaceRequest: MusicWorkspaceActionRequest = {
      ...request,
      draft: {
        ...request.draft,
        outputFormat: "m4a",
        title: "Hook fix",
      },
      operationId: "replace_section",
      parameters: {
        infillLyrics: "new hook lyric",
        negativeTags: "noise",
        prompt: "replace only the hook",
        rangeEndSec: 36,
        rangeStartSec: 12,
      },
      sourceTrackIds: ["track-a"],
    };

    expect(buildMusicPrepareBody(replaceRequest, tracks, {
      enabled: true,
      id: "replace_section",
      requirement: { minTracks: 1, maxTracks: 1 },
      supportsAudioFormat: true,
      supportsMaxMode: true,
    }).music).toEqual(expect.objectContaining({
      action: "replace_section",
      audio_format: "m4a",
      end_sec: 36,
      infill_lyrics: "new hook lyric",
      negative_tags: "noise",
      prompt: "replace only the hook",
      start_sec: 12,
      title: "Hook fix",
    }));

    expect(buildMusicPrepareBody({
      ...request,
      operationId: "export",
      parameters: {},
    }, tracks, { enabled: true, id: "export" }).music).toEqual(expect.objectContaining({
      action: "export",
      format: "wav",
      formats: ["wav"],
    }));

    expect(buildMusicPrepareBody({
      ...request,
      draft: { ...request.draft, lyrics: "own lyric" },
      mode: "own_lyrics",
      operationId: "extend",
      parameters: { prompt: "continue the outro", seekSec: 32 },
      sourceTrackIds: ["track-a"],
    }, tracks, {
      enabled: true,
      id: "extend",
      requirement: { minTracks: 1, maxTracks: 1 },
    }).music).not.toHaveProperty("custom");
  });
});
