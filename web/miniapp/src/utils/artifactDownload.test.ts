import { describe, expect, test } from "vitest";

import { neirohubArtifactFilename } from "./artifactDownload";

const JOB_ID = "550e8400-e29b-41d4-a716-446655440000";

describe("artifact filename helper", () => {
  test("builds a bounded safe slug from prompt text", () => {
    const filename = neirohubArtifactFilename("  Neon cat!!! on   Mars  ", JOB_ID, "image/png");

    expect(filename).toBe("Neirohub_neon_cat_on_mars.png");
    expect(filename).not.toContain("!");
    expect(filename).not.toContain(" ");
  });

  test("falls back to job id prefix when prompt is empty", () => {
    expect(neirohubArtifactFilename("   ", JOB_ID, "video/mp4")).toBe("Neirohub_550e8400.mp4");
  });

  test("uses fallback extension when mime is absent", () => {
    expect(neirohubArtifactFilename("file", JOB_ID, undefined, "webp")).toBe("Neirohub_file.webp");
  });
});
