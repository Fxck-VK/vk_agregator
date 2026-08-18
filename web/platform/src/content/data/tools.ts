import "server-only";

import type { ToolContent } from "../domain/types";

export const tools = [
  {
    id: "tool-image-generator",
    kind: "tool",
    status: "published",
    indexing: "index",
    createdAt: "2026-08-10T09:00:00+03:00",
    updatedAt: "2026-08-15T12:00:00+03:00",
    publishedAt: "2026-08-14T10:00:00+03:00",
    lastReviewedAt: "2026-08-15T11:00:00+03:00",
    author: "NeiroHub editorial",
    reviewer: "NeiroHub review",
    taskCategory: "image",
    workspacePath: "/app/image",
    relatedModelIds: ["model-gpt-image-2"],
    relatedArticleIds: ["article-image-prompt-basics"],
    translations: {
      ru: {
        slug: "generator-izobrazheniy",
        legacySlugs: ["sozdat-izobrazhenie"],
        title: "Генератор изображений",
        shortDescription: "Рабочий инструмент для создания изображения по описанию и выбранным настройкам.",
        seo: {
          title: "Генератор изображений NeiroHub",
          description: "Создавайте изображения по текстовому описанию, выбирайте модель и контролируйте параметры запуска.",
        },
        inputs: ["Текстовое описание изображения", "Необязательное референсное изображение"],
        outputs: ["Готовое изображение в библиотеке файлов"],
        useCases: ["Обложки", "Иллюстрации", "Концепты для рекламных материалов"],
        limitations: ["Доступные параметры зависят от выбранной серверной модели"],
        workflowSteps: [
          "Сформулируйте желаемый результат",
          "Выберите доступную модель и параметры",
          "Проверьте серверную стоимость и подтвердите запуск",
        ],
      },
      en: {
        slug: "image-generator",
        legacySlugs: ["create-an-image"],
        title: "Image generator",
        shortDescription: "A workspace tool for creating an image from a description and selected settings.",
        seo: {
          title: "NeiroHub image generator",
          description: "Create images from text, choose an available model, and control generation settings on NeiroHub.",
        },
        inputs: ["Text image description", "Optional reference image"],
        outputs: ["A completed image in the file library"],
        useCases: ["Covers", "Illustrations", "Advertising concepts"],
        limitations: ["Available settings depend on the selected server model"],
        workflowSteps: [
          "Describe the desired result",
          "Choose an available model and settings",
          "Check the server-provided price and confirm the job",
        ],
      },
    },
  },
] satisfies ToolContent[];
