import { z } from "zod";

const nonEmptyTextSchema = z.string().trim().min(1);
const timestampSchema = z.string().datetime({ offset: true });
export const contentSlugSchema = z
  .string()
  .trim()
  .regex(/^[a-z0-9]+(?:-[a-z0-9]+)*$/, "Slug must contain lowercase ASCII words separated by hyphens.");

const modelIdSchema = z.string().regex(/^model-[a-z0-9]+(?:-[a-z0-9]+)*$/);
const toolIdSchema = z.string().regex(/^tool-[a-z0-9]+(?:-[a-z0-9]+)*$/);
const articleIdSchema = z.string().regex(/^article-[a-z0-9]+(?:-[a-z0-9]+)*$/);
const contentIdSchema = z.string().regex(/^(?:model|tool|article)-[a-z0-9]+(?:-[a-z0-9]+)*$/);

function uniqueStringArray<T extends z.ZodType<string>>(itemSchema: T) {
  return z
    .array(itemSchema)
    .refine((items) => new Set(items).size === items.length, "Values must be unique.");
}

const localizedSeoSchema = z.strictObject({
  title: nonEmptyTextSchema,
  description: nonEmptyTextSchema,
});

const localizedMetadataFields = {
  slug: contentSlugSchema,
  legacySlugs: uniqueStringArray(contentSlugSchema),
  title: nonEmptyTextSchema,
  shortDescription: nonEmptyTextSchema,
  seo: localizedSeoSchema,
} as const;

function validateLocalizedSlugs(
  translation: { slug: string; legacySlugs: string[] },
  context: z.RefinementCtx,
) {
  if (translation.legacySlugs.includes(translation.slug)) {
    context.addIssue({
      code: "custom",
      path: ["legacySlugs"],
      message: "Current slug must not be repeated as a legacy slug.",
    });
  }
}

export const modelTranslationSchema = z
  .strictObject({
    ...localizedMetadataFields,
    capabilities: z.array(nonEmptyTextSchema).min(1),
    useCases: z.array(nonEmptyTextSchema).min(1),
    limitations: z.array(nonEmptyTextSchema).min(1),
    workflowNotes: z.array(nonEmptyTextSchema).min(1),
  })
  .superRefine(validateLocalizedSlugs);

export const toolTranslationSchema = z
  .strictObject({
    ...localizedMetadataFields,
    inputs: z.array(nonEmptyTextSchema).min(1),
    outputs: z.array(nonEmptyTextSchema).min(1),
    useCases: z.array(nonEmptyTextSchema).min(1),
    limitations: z.array(nonEmptyTextSchema).min(1),
    workflowSteps: z.array(nonEmptyTextSchema).min(1),
  })
  .superRefine(validateLocalizedSlugs);

export const articleContentBlockSchema = z.discriminatedUnion("type", [
  z.strictObject({
    type: z.literal("heading"),
    level: z.union([z.literal(2), z.literal(3)]),
    text: nonEmptyTextSchema,
  }),
  z.strictObject({
    type: z.literal("paragraph"),
    text: nonEmptyTextSchema,
  }),
  z.strictObject({
    type: z.literal("list"),
    style: z.enum(["ordered", "unordered"]),
    items: z.array(nonEmptyTextSchema).min(1),
  }),
]);

export const articleTranslationSchema = z
  .strictObject({
    ...localizedMetadataFields,
    body: z.array(articleContentBlockSchema).min(1),
  })
  .superRefine(validateLocalizedSlugs);

const contentStatusSchema = z.enum(["draft", "review", "published", "archived"]);
const indexingPolicySchema = z.enum(["index", "noindex"]);
const contentCategorySchema = z.enum(["text", "image", "video", "audio", "multimodal"]);

const lifecycleFields = {
  status: contentStatusSchema,
  indexing: indexingPolicySchema,
  createdAt: timestampSchema,
  updatedAt: timestampSchema,
  publishedAt: timestampSchema.optional(),
  lastReviewedAt: timestampSchema.optional(),
  author: nonEmptyTextSchema,
  reviewer: nonEmptyTextSchema.optional(),
} as const;

type LifecycleRecord = {
  status: z.infer<typeof contentStatusSchema>;
  indexing: z.infer<typeof indexingPolicySchema>;
  createdAt: string;
  updatedAt: string;
  publishedAt?: string;
  lastReviewedAt?: string;
  reviewer?: string;
  translations: { ru?: unknown; en?: unknown };
};

function validateLifecycle(record: LifecycleRecord, context: z.RefinementCtx) {
  if (Date.parse(record.updatedAt) < Date.parse(record.createdAt)) {
    context.addIssue({
      code: "custom",
      path: ["updatedAt"],
      message: "updatedAt must not be earlier than createdAt.",
    });
  }

  if (record.publishedAt && Date.parse(record.publishedAt) < Date.parse(record.createdAt)) {
    context.addIssue({
      code: "custom",
      path: ["publishedAt"],
      message: "publishedAt must not be earlier than createdAt.",
    });
  }

  if (record.lastReviewedAt && Date.parse(record.lastReviewedAt) < Date.parse(record.createdAt)) {
    context.addIssue({
      code: "custom",
      path: ["lastReviewedAt"],
      message: "lastReviewedAt must not be earlier than createdAt.",
    });
  }

  if (record.status !== "published" && record.indexing !== "noindex") {
    context.addIssue({
      code: "custom",
      path: ["indexing"],
      message: "Only published content may be indexable.",
    });
  }

  if (record.status === "review" || record.status === "published") {
    if (!record.translations.ru) {
      context.addIssue({
        code: "custom",
        path: ["translations", "ru"],
        message: "Review and published content requires a Russian translation.",
      });
    }

    if (!record.translations.en) {
      context.addIssue({
        code: "custom",
        path: ["translations", "en"],
        message: "Review and published content requires an English translation.",
      });
    }
  }

  if (record.status === "published") {
    if (!record.reviewer) {
      context.addIssue({ code: "custom", path: ["reviewer"], message: "Published content requires a reviewer." });
    }
    if (!record.publishedAt) {
      context.addIssue({
        code: "custom",
        path: ["publishedAt"],
        message: "Published content requires publishedAt.",
      });
    }
    if (!record.lastReviewedAt) {
      context.addIssue({
        code: "custom",
        path: ["lastReviewedAt"],
        message: "Published content requires lastReviewedAt.",
      });
    }
  }
}

const localizedModelTranslationsSchema = z.strictObject({
  ru: modelTranslationSchema.optional(),
  en: modelTranslationSchema.optional(),
});

const localizedToolTranslationsSchema = z.strictObject({
  ru: toolTranslationSchema.optional(),
  en: toolTranslationSchema.optional(),
});

const localizedArticleTranslationsSchema = z.strictObject({
  ru: articleTranslationSchema.optional(),
  en: articleTranslationSchema.optional(),
});

export const modelContentSchema = z
  .strictObject({
    id: modelIdSchema,
    kind: z.literal("model"),
    ...lifecycleFields,
    category: contentCategorySchema,
    runtimeCatalogKey: contentSlugSchema.optional(),
    relatedToolIds: uniqueStringArray(contentIdSchema),
    relatedArticleIds: uniqueStringArray(contentIdSchema),
    translations: localizedModelTranslationsSchema,
  })
  .superRefine(validateLifecycle);

export const toolContentSchema = z
  .strictObject({
    id: toolIdSchema,
    kind: z.literal("tool"),
    ...lifecycleFields,
    taskCategory: contentCategorySchema,
    workspacePath: z
      .string()
      .regex(/^\/app(?:\/[a-z0-9][a-z0-9/_-]*)?$/, "Workspace path must be a safe same-origin /app path.")
      .optional(),
    relatedModelIds: uniqueStringArray(contentIdSchema),
    relatedArticleIds: uniqueStringArray(contentIdSchema),
    translations: localizedToolTranslationsSchema,
  })
  .superRefine(validateLifecycle);

export const articleContentSchema = z
  .strictObject({
    id: articleIdSchema,
    kind: z.literal("article"),
    ...lifecycleFields,
    category: z.enum(["guide", "news", "comparison", "tutorial"]),
    relatedModelIds: uniqueStringArray(contentIdSchema),
    relatedToolIds: uniqueStringArray(contentIdSchema),
    translations: localizedArticleTranslationsSchema,
  })
  .superRefine(validateLifecycle);

export const contentGraphSchema = z.strictObject({
  models: z.array(modelContentSchema),
  tools: z.array(toolContentSchema),
  articles: z.array(articleContentSchema),
});
