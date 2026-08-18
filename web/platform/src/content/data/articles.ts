import "server-only";

import type { ArticleContent } from "../domain/types";

export const articles = [
  {
    id: "article-image-prompt-basics",
    kind: "article",
    status: "published",
    indexing: "index",
    createdAt: "2026-08-10T09:00:00+03:00",
    updatedAt: "2026-08-15T12:00:00+03:00",
    publishedAt: "2026-08-14T10:00:00+03:00",
    lastReviewedAt: "2026-08-15T11:00:00+03:00",
    author: "NeiroHub editorial",
    reviewer: "NeiroHub review",
    category: "guide",
    relatedModelIds: ["model-gpt-image-2"],
    relatedToolIds: ["tool-image-generator"],
    translations: {
      ru: {
        slug: "osnovy-promtov-dlya-izobrazheniy",
        legacySlugs: ["kak-napisat-promt-dlya-kartinki"],
        title: "Основы промптов для изображений",
        shortDescription: "Практическая структура запроса, которая помогает точнее описать желаемый результат.",
        seo: {
          title: "Как составить промпт для генерации изображения",
          description: "Разберите структуру понятного промпта: объект, действие, окружение, композиция и визуальный стиль.",
        },
        body: [
          {
            type: "paragraph",
            text: "Хороший запрос описывает не набор случайных тегов, а цельную сцену и важные ограничения результата.",
          },
          { type: "heading", level: 2, text: "Начните с главного" },
          {
            type: "list",
            style: "unordered",
            items: [
              "Назовите главный объект и его действие",
              "Опишите окружение и композицию",
              "Уточните свет, цвет и визуальный стиль",
            ],
          },
          {
            type: "paragraph",
            text: "После первого результата меняйте по одному смысловому элементу, чтобы понимать влияние каждого уточнения.",
          },
        ],
      },
      en: {
        slug: "image-prompt-basics",
        legacySlugs: ["how-to-write-an-image-prompt"],
        title: "Image prompt basics",
        shortDescription: "A practical request structure that helps describe the desired result more precisely.",
        seo: {
          title: "How to write an image generation prompt",
          description: "Learn a clear prompt structure: subject, action, environment, composition, and visual style.",
        },
        body: [
          {
            type: "paragraph",
            text: "A useful request describes a coherent scene and its important constraints instead of listing unrelated tags.",
          },
          { type: "heading", level: 2, text: "Start with the essentials" },
          {
            type: "list",
            style: "unordered",
            items: [
              "Name the main subject and action",
              "Describe the environment and composition",
              "Specify lighting, color, and visual style",
            ],
          },
          {
            type: "paragraph",
            text: "After the first result, change one meaningful element at a time so you can judge each refinement.",
          },
        ],
      },
    },
  },
  {
    id: "article-content-workflow-draft",
    kind: "article",
    status: "draft",
    indexing: "noindex",
    createdAt: "2026-08-16T09:00:00+03:00",
    updatedAt: "2026-08-16T09:00:00+03:00",
    author: "NeiroHub editorial",
    category: "guide",
    relatedModelIds: [],
    relatedToolIds: [],
    translations: {
      ru: {
        slug: "redakcionnyj-process-neirohub",
        legacySlugs: [],
        title: "Редакционный процесс NeiroHub",
        shortDescription: "Черновик внутреннего описания подготовки публичных материалов.",
        seo: {
          title: "Редакционный процесс NeiroHub",
          description: "Черновой материал о проверке и публикации контента NeiroHub.",
        },
        body: [
          {
            type: "paragraph",
            text: "Материал проходит редакционную и техническую проверку до публикации.",
          },
        ],
      },
    },
  },
] satisfies ArticleContent[];
