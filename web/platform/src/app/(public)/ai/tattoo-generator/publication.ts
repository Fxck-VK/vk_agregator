import "server-only";
import type { PublicPagePublication } from "@/content/publication/types";

export const tattooPublication: PublicPagePublication = { id: "tattoo-generator", status: "draft", indexing: "noindex", paths: { en: "/ai/tattoo-generator", ru: "/ai/tattoo-generator" } };
export const publications = [tattooPublication];
