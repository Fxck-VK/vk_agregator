import "server-only";

import type { ModelContent } from "../domain/types";

export const models = [
  {
    id: "model-gpt-image-2",
    kind: "model",
    status: "published",
    indexing: "index",
    createdAt: "2026-08-10T09:00:00+03:00",
    updatedAt: "2026-08-15T12:00:00+03:00",
    publishedAt: "2026-08-14T10:00:00+03:00",
    lastReviewedAt: "2026-08-15T11:00:00+03:00",
    author: "NeiroHub editorial",
    reviewer: "NeiroHub review",
    category: "image",
    runtimeCatalogKey: "gpt-image-2",
    relatedToolIds: ["tool-image-generator"],
    relatedArticleIds: ["article-image-prompt-basics"],
    translations: {
      ru: {
        slug: "gpt-image-2",
        legacySlugs: ["generator-izobrazheniy-gpt-image-2"],
        title: "GPT Image 2",
        shortDescription: "Модель для создания и редактирования изображений по текстовому запросу.",
        seo: {
          title: "GPT Image 2 в NeiroHub",
          description: "Возможности GPT Image 2, примеры задач и понятный процесс создания изображений в NeiroHub.",
        },
        capabilities: [
          "Создание нового изображения по текстовому описанию",
          "Редактирование изображения с сохранением основной композиции",
        ],
        useCases: [
          "Иллюстрации для публикаций и презентаций",
          "Поиск визуальной концепции до работы с дизайнером",
        ],
        limitations: [
          "Модель может неточно воспроизводить мелкий текст и сложные детали",
          "Готовый результат необходимо проверять перед публикацией",
        ],
        workflowNotes: [
          "Опишите главный объект, действие, окружение и визуальный стиль",
          "После генерации уточняйте только те детали, которые нужно изменить",
        ],
      },
      en: {
        slug: "gpt-image-2",
        legacySlugs: ["openai-image-generator"],
        title: "GPT Image 2",
        shortDescription: "A model for creating and editing images from a text request.",
        seo: {
          title: "GPT Image 2 on NeiroHub",
          description: "Explore GPT Image 2 capabilities, suitable tasks, and a clear image creation workflow on NeiroHub.",
        },
        capabilities: [
          "Create a new image from a text description",
          "Edit an image while preserving its main composition",
        ],
        useCases: [
          "Illustrations for articles and presentations",
          "Visual concept exploration before working with a designer",
        ],
        limitations: [
          "The model may reproduce small text and complex details inaccurately",
          "Review generated output before publishing it",
        ],
        workflowNotes: [
          "Describe the main subject, action, environment, and visual style",
          "After generation, specify only the details that need to change",
        ],
      },
    },
  },
] satisfies ModelContent[];
