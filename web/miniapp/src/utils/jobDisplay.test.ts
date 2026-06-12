import { describe, expect, test } from "vitest";

import type { Job } from "../api/client";
import { dedupeHistoryJobs, historyCountLabel, jobDisplayTitle, truncateTitle } from "./jobDisplay";

function job(overrides: Partial<Job>): Job {
  return {
    id: "job-" + (overrides.id ?? "base"),
    operation: "text_generate",
    modality: "text",
    status: "succeeded",
    cost_estimate: 1,
    cost_captured: 1,
    output_artifact_ids: [],
    created_at: "2026-06-12T00:00:00Z",
    updated_at: "2026-06-12T00:00:00Z",
    ...overrides,
  };
}

describe("job display helpers", () => {
  test("truncates long titles predictably", () => {
    expect(truncateTitle("x".repeat(60), 8)).toBe("xxxxxxxx…");
  });

  test("uses prompt for history title when present", () => {
    expect(jobDisplayTitle(job({ prompt: "  short prompt  " }))).toBe("short prompt");
  });

  test("deduplicates chat jobs by conversation while keeping media jobs", () => {
    const jobs = [
      job({ id: "chat-new", conversation_id: "thread-1", created_at: "2026-06-12T00:02:00Z" }),
      job({ id: "chat-first", conversation_id: "thread-1", created_at: "2026-06-12T00:01:00Z" }),
      job({
        id: "image",
        operation: "image_generate",
        modality: "image",
        created_at: "2026-06-12T00:03:00Z",
      }),
    ];

    expect(dedupeHistoryJobs(jobs).map((item) => item.id)).toEqual(["image", "chat-first"]);
  });

  test("formats history count without throwing on common values", () => {
    expect(historyCountLabel(1)).toContain("1");
    expect(historyCountLabel(2)).toContain("2");
    expect(historyCountLabel(5)).toContain("5");
  });
});
