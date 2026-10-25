import { afterEach, describe, expect, it, vi } from "vitest";
import { clearImagePrompt, readImagePrompt, stageImagePrompt } from "./image-prompt-draft";

afterEach(() => { vi.restoreAllMocks(); clearImagePrompt(readImagePrompt()); });
describe("transient image prompt handoff", () => {
  it("expires an abandoned draft after five minutes", () => {
    const now = vi.spyOn(Date, "now").mockReturnValue(1000);
    stageImagePrompt("A paper bird");
    expect(readImagePrompt()).toBe("A paper bird");
    now.mockReturnValue(301001);
    expect(readImagePrompt()).toBe("");
  });
  it("does not clear a newer draft when an earlier consumer finishes", () => {
    stageImagePrompt("First idea"); stageImagePrompt("Second idea");
    clearImagePrompt("First idea"); expect(readImagePrompt()).toBe("Second idea");
    clearImagePrompt("Second idea"); expect(readImagePrompt()).toBe("");
  });
});
