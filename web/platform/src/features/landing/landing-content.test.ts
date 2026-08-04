import { describe, expect, it } from "vitest";

import {
  landingCapabilities,
  landingFaq,
  landingFooterGroups,
  landingModels,
  landingNews,
  landingTools,
} from "./landing-content";

const supportedInternalTargets = new Set([
  "/",
  "/login?next=/app",
  "/login?next=/app/chats",
  "/login?next=/app/files",
  "/login?next=/app/image",
  "/login?next=/app/inspiration",
  "/login?next=/app/models",
]);

describe("landing content", () => {
  it("keeps every content id unique within its collection", () => {
    for (const collection of [landingTools, landingNews, landingModels, landingCapabilities, landingFaq]) {
      expect(new Set(collection.map(({ id }) => id)).size).toBe(collection.length);
    }
  });

  it("provides useful alternative text for every editorial image", () => {
    const imageContent = [...landingNews, ...landingCapabilities];

    expect(imageContent.length).toBeGreaterThan(0);
    for (const item of imageContent) {
      expect(item.imageAlt.trim().length).toBeGreaterThan(10);
    }
  });

  it("only links to public or currently supported authenticated targets", () => {
    const hrefs = [
      ...landingTools.map(({ href }) => href),
      ...landingNews.map(({ href }) => href),
      ...landingModels.map(({ href }) => href),
      ...landingCapabilities.map(({ href }) => href),
      ...landingFooterGroups.flatMap(({ links }) => links.map(({ href }) => href)),
    ];

    for (const href of hrefs) {
      expect(supportedInternalTargets.has(href), `unsupported landing href: ${href}`).toBe(true);
    }
  });

  it("does not publish unverified ratings or usage counters", () => {
    for (const model of landingModels) {
      expect(model).not.toHaveProperty("rating");
      expect(model).not.toHaveProperty("usageCount");
    }
  });

  it("contains enough content for the approved initial and expanded states", () => {
    expect(landingTools).toHaveLength(7);
    expect(landingModels).toHaveLength(10);
    expect(landingFaq).toHaveLength(5);
    expect(landingFooterGroups.length).toBeGreaterThanOrEqual(3);
  });
});
