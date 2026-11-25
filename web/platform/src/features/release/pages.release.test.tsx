import { afterEach, describe, expect, it, vi } from "vitest";
vi.mock("server-only", () => ({}));
const state = vi.hoisted(() => ({ view: "prod" as "prod" | "dev", localSimulation: false }));
vi.mock("@/features/release/server", () => ({ getRequestReleaseState: async () => ({ view: state.view, canSwitch: state.view === "dev", localSimulation: state.localSimulation }) }));
vi.mock("@/i18n/server", () => ({ getRequestLocale: async () => "ru", notFound: () => { throw new Error("blocked404"); } }));
vi.mock("@/features/presentations/access.server", () => ({ loadPresentationAccess: async () => ({ kind: "guest" }), presentationApiReachable: async () => false }));
import { contentMetadata, renderContentPage, renderContentRecord } from "@/content/templates/render";
import { contentPages } from "@/content/templates/catalog";
import PresentationsPage, { generateMetadata as presentationsMetadata } from "@/app/(public)/workflows/presentations/page";
const presentationPage = (query = {}) => PresentationsPage({ searchParams: Promise.resolve(query) });
afterEach(() => { state.view = "prod"; state.localSimulation = false; });
describe("public route admission", () => {
  it.each(["/mcp", "/ai/image-generator", "/business", "/workflows/product-campaign"])("blocks draft route and metadata in PROD: %s", async path => {
    await expect(renderContentPage(path)).rejects.toThrow("blocked404");
    await expect(contentMetadata(path)).rejects.toThrow("blocked404");
  });
  it("does not allow direct renderer calls to bypass publication", async () => {
    await expect(renderContentRecord(contentPages.find(p => p.template === "T03")!)).rejects.toThrow("blocked404");
  });
  it.each(["prod", "dev"] as const)("serves the homepage in %s while keeping search indexing disabled", async view => {
    state.view = view;
    await expect(contentMetadata("/")).resolves.toMatchObject({ robots: { index: false, follow: false } });
    await expect(renderContentPage("/")).resolves.toBeDefined();
  });
  it("keeps DEV draft metadata readable", async () => {
    state.view = "dev";
    await expect(contentMetadata("/ai/image-generator")).resolves.toMatchObject({ robots: { index: false, follow: false } });
  });
  it("exposes the presentation brief only in DEV and keeps its metadata out of indexing", async () => {
    await expect(presentationPage()).rejects.toThrow("blocked404");
    await expect(presentationsMetadata()).rejects.toThrow("blocked404");
    state.view = "dev";
    await expect(presentationPage()).resolves.toBeDefined();
    await expect(presentationsMetadata()).resolves.toMatchObject({ robots: { index: false, follow: false } });
  });
  it("preserves a valid presentation run only through the DEV page", async () => {
    const conversation = "12345678-0000-4000-8000-012345678901";
    const job = "12345678-0000-4000-8000-012345678902";
    await expect(presentationPage({ run: conversation, job })).rejects.toThrow("blocked404");
    state.view = "dev";
    await expect(presentationPage({ run: conversation, job })).resolves.toMatchObject({ props: { run: { conversation, job } } });
    await expect(presentationPage({ run: "unsafe" })).resolves.toMatchObject({ props: { run: undefined } });
  });
  it("offers the fixed example to guests only in local simulation", async () => {
    state.view = "dev";
    await expect(presentationPage()).resolves.toMatchObject({ props: { access: { kind: "guest" }, showLocalExample: false } });
    state.localSimulation = true;
    await expect(presentationPage()).resolves.toMatchObject({ props: { access: { kind: "guest" }, showLocalExample: true } });
    state.view = "prod";
    await expect(presentationPage()).rejects.toThrow("blocked404");
  });
});
