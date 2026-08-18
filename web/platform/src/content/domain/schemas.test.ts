import { describe, expect, it } from "vitest";

import {
  articleContentSchema,
  modelContentSchema,
  toolContentSchema,
} from "./schemas";

const timestamps = {
  createdAt: "2026-08-01T09:00:00+03:00",
  updatedAt: "2026-08-03T12:00:00+03:00",
  publishedAt: "2026-08-02T10:00:00+03:00",
  lastReviewedAt: "2026-08-03T11:00:00+03:00",
};

const modelTranslations = {
  ru: {
    slug: "generator-izobrazheniy",
    legacySlugs: ["sozdanie-izobrazheniy"],
    title: "Генератор изображений",
    shortDescription: "Создание изображений по текстовому описанию.",
    seo: {
      title: "Генератор изображений NeiroHub",
      description: "Создавайте изображения по текстовому описанию в NeiroHub.",
    },
    capabilities: ["Создание изображения по тексту"],
    useCases: ["Иллюстрации для публикаций"],
    limitations: ["Результат необходимо проверять"],
    workflowNotes: ["Сначала опишите сцену и композицию"],
  },
  en: {
    slug: "image-generator-model",
    legacySlugs: ["create-images"],
    title: "Image generator",
    shortDescription: "Create images from a text description.",
    seo: {
      title: "NeiroHub image generator",
      description: "Create images from text descriptions with NeiroHub.",
    },
    capabilities: ["Text-to-image generation"],
    useCases: ["Editorial illustrations"],
    limitations: ["Generated output requires review"],
    workflowNotes: ["Describe the scene and composition first"],
  },
};

const validPublishedModel = {
  id: "model-image-generator",
  kind: "model",
  status: "published",
  indexing: "index",
  ...timestamps,
  author: "NeiroHub editorial",
  reviewer: "NeiroHub review",
  category: "image",
  relatedToolIds: ["tool-image-generator"],
  relatedArticleIds: ["article-image-prompt-basics"],
  translations: modelTranslations,
};

const validDraftModel = {
  ...validPublishedModel,
  status: "draft",
  indexing: "noindex",
  publishedAt: undefined,
  lastReviewedAt: undefined,
  reviewer: undefined,
  translations: { ru: modelTranslations.ru },
};

const validPublishedTool = {
  id: "tool-image-generator",
  kind: "tool",
  status: "published",
  indexing: "index",
  ...timestamps,
  author: "NeiroHub editorial",
  reviewer: "NeiroHub review",
  taskCategory: "image",
  workspacePath: "/app/image",
  relatedModelIds: ["model-image-generator"],
  relatedArticleIds: ["article-image-prompt-basics"],
  translations: {
    ru: {
      slug: "generator-izobrazheniy",
      legacySlugs: ["sozdanie-izobrazheniy"],
      title: "Генератор изображений",
      shortDescription: "Создание изображений по текстовому описанию.",
      seo: {
        title: "Генератор изображений NeiroHub",
        description: "Создавайте изображения по текстовому описанию в NeiroHub.",
      },
      inputs: ["Текстовое описание"],
      outputs: ["Изображение"],
      useCases: ["Иллюстрации для публикаций"],
      limitations: ["Результат необходимо проверять"],
      workflowSteps: ["Опишите результат", "Запустите генерацию"],
    },
    en: {
      slug: "image-generator",
      legacySlugs: ["create-images"],
      title: "Image generator",
      shortDescription: "Create images from a text description.",
      seo: {
        title: "NeiroHub image generator",
        description: "Create images from text descriptions with NeiroHub.",
      },
      inputs: ["Text description"],
      outputs: ["Image"],
      useCases: ["Editorial illustrations"],
      limitations: ["Generated output requires review"],
      workflowSteps: ["Describe the result", "Start generation"],
    },
  },
};

const validPublishedArticle = {
  id: "article-image-prompt-basics",
  kind: "article",
  status: "published",
  indexing: "index",
  ...timestamps,
  author: "NeiroHub editorial",
  reviewer: "NeiroHub review",
  category: "guide",
  relatedModelIds: ["model-image-generator"],
  relatedToolIds: ["tool-image-generator"],
  translations: {
    ru: {
      slug: "osnovy-promtov-dlya-izobrazheniy",
      legacySlugs: [],
      title: "Основы промптов для изображений",
      shortDescription: "Как составлять понятные запросы для генератора.",
      seo: {
        title: "Как составить промпт для изображения",
        description: "Практическое руководство по запросам для генерации изображений.",
      },
      body: [
        { type: "heading", level: 2, text: "Опишите результат" },
        { type: "paragraph", text: "Начните с главного объекта и желаемого действия." },
        { type: "list", style: "unordered", items: ["Объект", "Действие", "Стиль"] },
      ],
    },
    en: {
      slug: "image-prompt-basics",
      legacySlugs: [],
      title: "Image prompt basics",
      shortDescription: "How to write clear requests for an image generator.",
      seo: {
        title: "How to write an image prompt",
        description: "A practical guide to prompts for image generation.",
      },
      body: [
        { type: "heading", level: 2, text: "Describe the result" },
        { type: "paragraph", text: "Start with the main subject and desired action." },
        { type: "list", style: "unordered", items: ["Subject", "Action", "Style"] },
      ],
    },
  },
};

describe("editorial content schemas", () => {
  it("accepts complete bilingual published model, tool, and article records", () => {
    expect(modelContentSchema.safeParse(validPublishedModel).success).toBe(true);
    expect(toolContentSchema.safeParse(validPublishedTool).success).toBe(true);
    expect(articleContentSchema.safeParse(validPublishedArticle).success).toBe(true);
  });

  it("allows an incomplete noindex draft", () => {
    expect(modelContentSchema.safeParse(validDraftModel).success).toBe(true);
  });

  it("requires both translations and review metadata for published content", () => {
    const result = modelContentSchema.safeParse({
      ...validPublishedModel,
      reviewer: undefined,
      translations: { ru: modelTranslations.ru },
    });

    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues.map((issue) => issue.path.join("."))).toEqual(
        expect.arrayContaining(["reviewer", "translations.en"]),
      );
    }
  });

  it("rejects indexable non-published content", () => {
    const result = modelContentSchema.safeParse({ ...validDraftModel, indexing: "index" });

    expect(result.success).toBe(false);
    if (!result.success) {
      expect(result.error.issues).toEqual([
        expect.objectContaining({ path: ["indexing"], message: "Only published content may be indexable." }),
      ]);
    }
  });

  it.each([
    "https://example.com/app/image",
    "//example.com/app/image",
    "/app/image?model=secret",
    "/app/image#generator",
    "javascript:alert(1)",
  ])("rejects unsafe workspace path %s", (workspacePath) => {
    expect(toolContentSchema.safeParse({ ...validPublishedTool, workspacePath }).success).toBe(false);
  });

  it("rejects non-URL-safe and duplicate localized slugs", () => {
    const result = modelContentSchema.safeParse({
      ...validPublishedModel,
      translations: {
        ...modelTranslations,
        en: {
          ...modelTranslations.en,
          slug: "Image Generator",
          legacySlugs: ["create-images", "create-images"],
        },
      },
    });

    expect(result.success).toBe(false);
  });

  it("rejects duplicate relationships and invalid lifecycle chronology", () => {
    const result = modelContentSchema.safeParse({
      ...validPublishedModel,
      updatedAt: "2026-07-31T12:00:00+03:00",
      relatedToolIds: ["tool-image-generator", "tool-image-generator"],
    });

    expect(result.success).toBe(false);
  });

  it("rejects trusted HTML blocks and volatile runtime facts", () => {
    const article = {
      ...validPublishedArticle,
      price: 55,
      translations: {
        ...validPublishedArticle.translations,
        en: {
          ...validPublishedArticle.translations.en,
          body: [{ type: "html", value: "<script>alert(1)</script>" }],
        },
      },
    };

    expect(articleContentSchema.safeParse(article).success).toBe(false);
    expect(modelContentSchema.safeParse({ ...validPublishedModel, balance: 1_000 }).success).toBe(false);
  });
});
