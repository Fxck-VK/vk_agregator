import "server-only";
import type { PublicPagePublication } from "@/content/publication/types";

export const toolsOverviewPublication: PublicPagePublication = {
  id: "ai-tools-overview",
  status: "draft",
  indexing: "noindex",
  paths: { en: "/ai", ru: "/ai" },
};
export const publications = [toolsOverviewPublication];
