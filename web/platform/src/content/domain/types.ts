import type { z } from "zod";

import type {
  articleContentBlockSchema,
  articleContentSchema,
  articleTranslationSchema,
  contentGraphSchema,
  modelContentSchema,
  modelTranslationSchema,
  toolContentSchema,
  toolTranslationSchema,
} from "./schemas";

export type ArticleContentBlock = z.infer<typeof articleContentBlockSchema>;
export type ModelTranslation = z.infer<typeof modelTranslationSchema>;
export type ToolTranslation = z.infer<typeof toolTranslationSchema>;
export type ArticleTranslation = z.infer<typeof articleTranslationSchema>;
export type ModelContent = z.infer<typeof modelContentSchema>;
export type ToolContent = z.infer<typeof toolContentSchema>;
export type ArticleContent = z.infer<typeof articleContentSchema>;
export type ContentGraph = z.infer<typeof contentGraphSchema>;
export type ContentStatus = ModelContent["status"];
export type IndexingPolicy = ModelContent["indexing"];
