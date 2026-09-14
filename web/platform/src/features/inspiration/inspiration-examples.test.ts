import { describe, expect, it } from "vitest";

import {
  inspirationExamples,
  selectInspirationExamples,
} from "./inspiration-examples";

describe("selectInspirationExamples", () => {
  it("includes the complete uploaded inspiration collection", () => {
    expect(inspirationExamples).toHaveLength(13);
    expect(inspirationExamples).toEqual(expect.arrayContaining([
      expect.objectContaining({ mediaType: "image", mediaPath: "/assets/images/inspiration/sleeping-on-cloud.png" }),
      expect.objectContaining({ mediaType: "video", mediaPath: "/assets/videos/inspiration/video-1.mp4" }),
      expect.objectContaining({ mediaType: "video", mediaPath: "/assets/videos/inspiration/video-2.mp4" }),
    ]));
    expect(inspirationExamples.filter((example) => example.mediaType === "image")).toHaveLength(11);
    expect(inspirationExamples.filter((example) => example.mediaType === "video")).toHaveLength(2);
  });

  it("returns examples associated with the selected model", () => {
    expect(selectInspirationExamples("gpt_image_2")).toEqual([
      expect.objectContaining({ modelId: "gpt_image_2" }),
    ]);
  });

  it("uses the shared collection when the selected model has no dedicated examples", () => {
    expect(selectInspirationExamples("nano-banana-pro")).toEqual(inspirationExamples.slice(0, 3));
  });

  it("can limit the shared collection to image examples", () => {
    const imageExamples = selectInspirationExamples(null, 20, "image");

    expect(imageExamples).toHaveLength(11);
    expect(imageExamples.every((example) => example.mediaType === "image")).toBe(true);
  });

  it("fills a short model selection with distinct photos from the shared collection", () => {
    const examples = selectInspirationExamples("gpt_image_2", 6, "image", { fillFromCollection: true });

    expect(examples).toHaveLength(6);
    expect(new Set(examples.map((example) => example.id)).size).toBe(6);
    expect(examples[0].modelId).toBe("gpt_image_2");
    expect(examples.every((example) => example.mediaType === "image")).toBe(true);
  });
});
