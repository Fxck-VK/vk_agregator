import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { inspirationExamples } from "../inspiration-examples";
import {
  INSPIRATION_IMAGE_SIZES,
  InspirationExampleMedia,
} from "./InspirationExampleMedia";

afterEach(cleanup);

describe("InspirationExampleMedia", () => {
  it("uses one stable optimized image request across every placement", () => {
    const example = inspirationExamples.find((item) => item.mediaType === "image")!;

    render(
      <>
        <InspirationExampleMedia className="gallery" example={example} />
        <InspirationExampleMedia className="template-picker" example={example} />
        <InspirationExampleMedia className="generation-example" example={example} />
      </>,
    );

    const images = screen.getAllByRole("img", { name: example.mediaAlt });
    const sources = images.map((image) => image.getAttribute("src"));
    const sourceSets = images.map((image) => image.getAttribute("srcset"));

    expect(new Set(sources).size).toBe(1);
    expect(new Set(sourceSets).size).toBe(1);
    for (const image of images) {
      expect(image).toHaveAttribute("sizes", INSPIRATION_IMAGE_SIZES);
    }
  });
});
