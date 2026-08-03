import { describe, expect, it } from "vitest";

import { nextImageJobPollDelay } from "./image-job-polling";

describe("nextImageJobPollDelay", () => {
  it("caps the backoff after the last scheduled delay", () => {
    expect([0, 1, 2, 3, 4, 9].map(nextImageJobPollDelay)).toEqual([1500, 3000, 6000, 12000, 15000, 15000]);
  });

  it("uses the first delay for a negative attempt", () => {
    expect(nextImageJobPollDelay(-1)).toBe(1500);
  });
});
