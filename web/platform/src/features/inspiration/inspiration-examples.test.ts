import { describe, expect, it } from "vitest";

import {
  inspirationExamples,
  selectInspirationExamples,
} from "./inspiration-examples";

describe("selectInspirationExamples", () => {
  it("returns examples associated with the selected model", () => {
    expect(selectInspirationExamples("gpt-image-2")).toEqual([
      expect.objectContaining({ modelId: "gpt-image-2" }),
    ]);
  });

  it("uses the shared collection when the selected model has no dedicated examples", () => {
    expect(selectInspirationExamples("nano-banana-pro")).toEqual(inspirationExamples);
  });
});
