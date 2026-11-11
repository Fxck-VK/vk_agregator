import { describe, expect, it } from "vitest";
import { featureEnabled, allowsReleasePath, releaseFeatures, releasePages } from "./policy";

describe("shared first-release policy", () => {
  it("keeps implemented core and excludes unfinished features in PROD", () => {
    expect(featureEnabled("images", "prod")).toBe(true);
    expect(featureEnabled("music", "prod")).toBe(false);
    expect(featureEnabled("mcp", "dev")).toBe(true);
    expect(featureEnabled("unknown", "dev")).toBe(false);
    expect(new Set(releaseFeatures.map(f => f.id)).size).toBe(releaseFeatures.length);
  });
  it("blocks direct and encoded unfinished routes while preserving account and legal", () => {
    for (const route of ["/ru/app/music", "/app/speech", "/ru/mcp", "/business", "/app/%6dusic"]) {
      expect(allowsReleasePath(route, "prod")).toBe(false);
      expect(allowsReleasePath(route, "dev")).toBe(true);
    }
    for (const route of ["/app", "/app/files", "/app/profile", "/login", "/legal", "/legal/paid-access"]) expect(allowsReleasePath(route, "prod")).toBe(true);
  });
  it("uses publication status for all24 SEO pages and preserves DEV drafts", () => {
    expect(releasePages).toHaveLength(24);
    expect(releasePages.filter(p => p.status === "published").map(p => p.path)).toEqual(["/"]);
    expect(releasePages.filter(p => p.path !== "/").every(p => p.status === "draft")).toBe(true);
    expect(allowsReleasePath("/ru", "prod")).toBe(true);
    expect(allowsReleasePath("/", "prod")).toBe(true);
    expect(allowsReleasePath("/ru/ai/image-generator", "prod")).toBe(false);
    expect(allowsReleasePath("/ru/ai/image-generator", "dev")).toBe(true);
    expect(allowsReleasePath("/academy/courses/epifera-essentials/choose-a-model", "prod")).toBe(false);
    expect(allowsReleasePath("javascript:alert(1)", "dev")).toBe(false);
  });
});
