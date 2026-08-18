import "server-only";

import type { ContentGraph } from "../domain/types";
import { articles } from "./articles";
import { models } from "./models";
import { tools } from "./tools";

export const rawContentGraph = {
  models,
  tools,
  articles,
} satisfies ContentGraph;
