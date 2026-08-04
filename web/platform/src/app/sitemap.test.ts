import { describe, expect, it } from "vitest";

import sitemap from "./sitemap";

describe("sitemap metadata route", () => {
  it("only includes the existing indexable homepage", () => {
    expect(sitemap()).toEqual([
      {
        url: "https://neiirohub.ru/",
        changeFrequency: "weekly",
        priority: 1,
      },
    ]);
  });
});
