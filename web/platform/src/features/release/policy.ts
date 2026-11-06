import { z } from "zod";
import registry from "../../../release/features.json";
import pages from "../../content/pages/catalog.json";

export type ReleaseView = "dev" | "prod";
export type ReleaseState = { view: ReleaseView; canSwitch: boolean; localSimulation: boolean };
export const releaseViewCookie = "epifera_release_view";
export const releaseViewHeader = "X-Epifera-Release-View";
const entry = z.object({ id: z.string().regex(/^[a-z][a-z-]*$/), name: z.string().min(1), status: z.enum(["ready", "needs-check", "not-connected"]), production: z.boolean(), path: z.string().startsWith("/"), works: z.string().min(1), remaining: z.string().min(1), verification: z.string().min(1) }).strict();
export const releaseFeatures = z.object({ version: z.literal(1), features: z.array(entry).min(1) }).strict().parse(registry).features;
if (new Set(releaseFeatures.map(f => f.id)).size !== releaseFeatures.length) throw new Error("Duplicate release feature");
export const releasePages = (pages as readonly { id: string; template: string; path: string; status: string; publishedAt?: string; seo: { title: string } }[]).map(p => ({ id: p.id, template: p.template, path: p.path, status: p.status, title: p.seo.title, publishedAt: p.publishedAt }));

export function featureEnabled(id: string, view: ReleaseView): boolean {
  const feature = releaseFeatures.find(f => f.id === id);
  return !!feature && (view === "dev" || feature.production);
}

function internalPath(href: string): string | null {
  if (href.startsWith("#") || href.startsWith("?")) return "";
  if (/^https?:\/\//i.test(href) || /^(mailto|tel):/i.test(href)) return "";
  if (!href.startsWith("/") || href.startsWith("//") || href.includes("\\")) return null;
  try {
    const path = decodeURIComponent(new URL(href, "https://release.invalid").pathname);
    if (path.includes("%") || path.includes("\\") || path.includes("//")) return null;
    return path.replace(/^\/(ru|en)(?=\/|$)/, "") || "/";
  } catch { return null; }
}

/** Presentation and page admission only. Backend ownership/pricing stay separate. */
export function allowsReleasePath(href: string, view: ReleaseView, now = Date.now()): boolean {
  const path = internalPath(href);
  if (path === null) return false;
  if (path === "" || view === "dev") return true;
  const is = (prefix: string) => path === prefix || path.startsWith(prefix + "/");
  if (is("/legal") || is("/login") || is("/invite")) return true;
  const routes = [["/app/music", "music"], ["/app/speech", "transcription"], ["/mcp", "mcp"], ["/business", "teams"], ["/workflows", "workflow-automation"]] as const;
  if (is("/app/ui-states") || routes.some(([prefix, id]) => is(prefix) && !featureEnabled(id, view))) return false;
  const record = releasePages.find(p => p.path === path || ["T10", "T11"].includes(p.template) && path.startsWith(p.path + "/"));
  return !record || record.status === "published" && (!record.publishedAt || Number.isFinite(Date.parse(record.publishedAt)) && Date.parse(record.publishedAt) <= now);
}

export function filterReleaseCatalog<T extends { default_model_id?: string; items: { id: string; verification?: string; kind?: string; operations?: { id: string; kind: string; enabled: boolean }[] }[] }>(catalog: T, view: ReleaseView): T {
  if (view === "dev") return catalog;
  const items = catalog.items.filter(model => model.verification !== "dev-smoke" && model.verification !== "pending-verification")
    .map(model => { const operations = model.operations?.filter(op => featureEnabled(op.kind === "audio" ? op.id === "transcribe" ? "transcription" : "music" : op.kind === "text" ? "chat" : op.kind === "image" ? "images" : "videos", view)); return operations?.length === model.operations?.length ? model : { ...model, operations }; })
    .filter(model => !model.operations || model.operations.some(op => op.enabled));
  if (items.length === catalog.items.length && items.every((model, index) => model === catalog.items[index])) return catalog;
  return { ...catalog, items, ...(catalog.default_model_id && !items.some(m => m.id === catalog.default_model_id) ? { default_model_id: items[0]?.id } : {}) };
}
