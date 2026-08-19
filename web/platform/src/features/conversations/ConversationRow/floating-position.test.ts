import { describe, expect, it } from "vitest";

import { resolveFloatingPosition } from "./floating-position";

describe("resolveFloatingPosition", () => {
  it("places the panel to the right of its trigger when there is room", () => {
    expect(resolveFloatingPosition(
      { bottom: 140, left: 250, right: 286, top: 104 },
      { height: 180, width: 240 },
      { height: 800, width: 1200 },
    )).toEqual({ left: 294, top: 104 });
  });

  it("opens upward and stays inside the viewport near the bottom edge", () => {
    expect(resolveFloatingPosition(
      { bottom: 756, left: 250, right: 286, top: 720 },
      { height: 220, width: 240 },
      { height: 768, width: 1200 },
    )).toEqual({ left: 294, top: 536 });
  });

  it("moves to the left and clamps to the viewport when the right side is too narrow", () => {
    expect(resolveFloatingPosition(
      { bottom: 140, left: 260, right: 296, top: 104 },
      { height: 180, width: 240 },
      { height: 640, width: 320 },
    )).toEqual({ left: 12, top: 104 });
  });
});
