import { describe, expect, it } from "vitest";
import { musicContinuationHref, parseMusicContinuation } from "./music-navigation";

describe("music continuation navigation", () => {
  it("binds a public model and original one-based track index to an application job", () => {
    const jobID = "11111111-1111-4111-8111-111111111111";
    const href = musicContinuationHref({ modelId: "suno_v6_wild", sourceJobId: jobID, originalIndex: 1 });
    expect(href).toContain("audio_index=2");
    const parsed = Object.fromEntries(new URLSearchParams(href!.split("?")[1]));
    expect(parseMusicContinuation(parsed)).toEqual({ modelId: "suno_v6_wild", jobId: jobID, audioIndex: 2 });
    for (const patch of [{ audio_index: "0" }, { source_job_id: "provider-native-task" }, { model: "lyria_3_5" }, { action: "upload" }]) {
      expect(parseMusicContinuation({ ...parsed, ...patch })).toBeUndefined();
    }
  });
});
