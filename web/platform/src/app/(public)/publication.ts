import "server-only";
import type { PublicPagePublication } from "@/content/publication/types";

export const homePublication: PublicPagePublication = {
  id: "public-home",
  status: "draft",
  indexing: "noindex",
  paths: { ru: "/", en: "/" },
};

export const publications = [homePublication];
