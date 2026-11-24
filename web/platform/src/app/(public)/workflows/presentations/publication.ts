import "server-only";

import type { PublicPagePublication } from "@/content/publication/types";
import { presentationWorkflowPath } from "@/features/presentations/route";

export const presentationWorkflowPublication: PublicPagePublication = {
  id: "epifera-presentation-workflow",
  status: "draft",
  indexing: "noindex",
  paths: { ru: presentationWorkflowPath },
};

export const publications = [presentationWorkflowPublication];
