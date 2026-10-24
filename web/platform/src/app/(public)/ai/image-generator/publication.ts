import "server-only";
import type { PublicPagePublication } from "@/content/publication/types";

// Keep the visual reference outside search until its own content is approved.
export const imageGeneratorPublication: PublicPagePublication = {
  id: "image-generator-demonstration",
  status: "draft",
  indexing: "noindex",
  paths: { ru: "/ai/image-generator", en: "/ai/image-generator" },
};

export const publications = [imageGeneratorPublication];
