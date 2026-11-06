import { afterEach, describe, expect, it, vi } from "vitest";
vi.mock("server-only", () => ({}));
const state = vi.hoisted(() => ({ view: "prod" as "prod" | "dev" }));
vi.mock("@/features/release/server", () => ({ getRequestReleaseState: async () => ({ view: state.view, canSwitch: state.view === "dev", localSimulation: false }) }));
vi.mock("@/i18n/server", () => ({ getRequestLocale: async () => "ru", notFound: () => { throw new Error("blocked404"); } }));
import { contentMetadata, renderContentPage, renderContentRecord } from "@/content/templates/render";
import { contentPages } from "@/content/templates/catalog";
afterEach(() => { state.view = "prod"; });
describe("public route admission", () => {
  it.each(["/", "/mcp", "/ai/image-generator", "/business", "/workflows/product-campaign"])("blocks draft route and metadata in PROD: %s", async path => {
    await expect(renderContentPage(path)).rejects.toThrow("blocked404");
    await expect(contentMetadata(path)).rejects.toThrow("blocked404");
  });
  it("does not allow direct renderer calls to bypass publication", async () => {
    await expect(renderContentRecord(contentPages.find(p => p.template === "T03")!)).rejects.toThrow("blocked404");
  });
  it("keeps DEV draft metadata readable", async () => {
    state.view = "dev";
    await expect(contentMetadata("/ai/image-generator")).resolves.toMatchObject({ robots: { index: false, follow: false } });
  });
});
