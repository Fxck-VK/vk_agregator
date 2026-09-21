import { afterEach, expect, it } from "vitest";
import { pendingImagePreview, readPendingMediaJob, savePendingMediaJob } from "./pending-media-job";

const jobID = "a2a006fc-4457-4bb5-bc4d-4f553d51766b";
afterEach(() => window.sessionStorage.clear());

it("keeps old records and video jobs compatible without image placeholders", () => {
  savePendingMediaJob("conversation", jobID, 3);
  expect(readPendingMediaJob("conversation")).toMatchObject({ jobID, baselineSeq: 3 });
  expect(readPendingMediaJob("conversation")?.image).toBeUndefined();
  expect(pendingImagePreview({ resolution: "1080p", duration_sec: 5, aspect_ratio: "9:16" })).toBeUndefined();
});

it("persists only count and ratio, not reference ids or the full request", () => {
  savePendingMediaJob("conversation", jobID, 3, { image_quality: "2K", output_count: 4, aspect_ratio: "9:16", reference_artifact_ids: [jobID] });
  expect(readPendingMediaJob("conversation")?.image).toEqual({ count: 4, aspectRatio: "9:16" });
  expect(window.sessionStorage.getItem("neirohub:conversation-pending-media:conversation")).not.toContain("reference_artifact_ids");
});

it("does not create unbounded grids from invalid stored data", () => {
  window.sessionStorage.setItem("neirohub:conversation-pending-media:conversation", JSON.stringify({ jobID, baselineSeq: 3, startedAt: Date.now(), image: { count: 1000000, aspectRatio: "1:0" } }));
  expect(readPendingMediaJob("conversation")).toBeNull();
});
