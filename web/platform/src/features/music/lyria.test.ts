import { describe, expect, it } from "vitest";
import { buildMusicPrepareBody } from "./music-api";
import type { MusicWorkspaceActionRequest } from "./MusicWorkspace";

describe("Lyria request", () => {
  it("drops cached Suno controls and bounds duration", () => {
    const request: MusicWorkspaceActionRequest = {
      modelId: "lyria_3_5", operationId: "generate", mode: "own_lyrics", sourceTrackIds: [], sourceJobIds: [], parameters: {},
      draft: { descriptionPrompt: "synthetic", lyrics: "test lyrics", style: "piano", title: "test", instrumental: false,
        maxMode: true, outputFormat: "mp3", promptWeight: 50, styleWeight: 50, variety: 50,
        targetDurationMode: "custom", targetDurationSec: 360 },
    };
    expect(buildMusicPrepareBody(request, [], undefined)).toEqual({ model_id: "lyria_3_5", sources: [], audio_artifact_ids: [],
      music: { action: "generate", prompt: "synthetic", lyrics: "test lyrics", style: "piano", title: "test", duration_sec: 240 } });
  });
});
