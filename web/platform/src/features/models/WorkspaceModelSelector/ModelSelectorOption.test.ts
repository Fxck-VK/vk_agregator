import { describe, expect, it } from "vitest";
import { getModelDescriptionPosition } from "./ModelSelectorOption";

describe("model description placement", () => {
  const anchor = { left: 112, right: 480, top: 300, bottom: 356 };
  const panel = { left: 100, right: 500, top: 100, bottom: 600 };
  const tooltip = { width: 280, height: 80 };

  it("places the description beside the panel when there is room", () => {
    expect(getModelDescriptionPosition(anchor, panel, tooltip, { width: 1280, height: 720 }))
      .toEqual({ left: 512, top: 288 });
  });

  it("flips to the left when the panel is at the right edge", () => {
    expect(getModelDescriptionPosition(
      { ...anchor, left: 800, right: 1230 },
      { ...panel, left: 768, right: 1264 },
      tooltip,
      { width: 1280, height: 720 },
    )).toEqual({ left: 476, top: 288 });
  });

  it("fits above the row on narrow screens", () => {
    expect(getModelDescriptionPosition(
      { ...anchor, left: 28, right: 360 },
      { ...panel, left: 16, right: 374 },
      tooltip,
      { width: 390, height: 667 },
    )).toEqual({ left: 28, top: 208 });
  });

  it("uses the space below the first row and clamps at the bottom edge", () => {
    expect(getModelDescriptionPosition(
      { left: 28, right: 280, top: 30, bottom: 86 },
      { left: 16, right: 304, top: 16, bottom: 500 },
      tooltip,
      { width: 320, height: 568 },
    )).toEqual({ left: 24, top: 98 });
    expect(getModelDescriptionPosition(
      { ...anchor, top: 680, bottom: 716 }, panel, tooltip, { width: 1280, height: 720 },
    )).toEqual({ left: 512, top: 624 });
  });
});
