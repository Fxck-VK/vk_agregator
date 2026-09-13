import "server-only";

import type {
  AccountProfile,
  ConversationMessageList,
  ConversationItem,
  ImageJobList,
  ImageJobResult,
  ImageModelList,
} from "../../lib/web-api/contracts";

export const localWorkspacePreviewProfile: AccountProfile = {
  account_id: "10000000-0000-4000-8000-000000000001",
  identity_refs: [
    {
      id: "10000000-0000-4000-8000-000000000002",
      account_id: "10000000-0000-4000-8000-000000000001",
      provider: "email",
      label: "preview@neirohub.local",
      verified: true,
      created_at: "2026-01-01T00:00:00Z",
    },
  ],
};

export const localWorkspacePreviewBalance = 1000;

export const localWorkspacePreviewChatModels = {
  default_model_id: "chatgpt",
  items: [{ id: "chatgpt", name: "NeiroHub Chat" }],
};

const localPreviewCranePrompt = "Создай фотореалистичное изображение: белый бумажный журавль парит на пушистом облаке в тёплой солнечной комнате с высоким арочным окном. Вертикальный кадр 2:3.";

export const localWorkspacePreviewConversations: ConversationItem[] = [
  {
    id: "20000000-0000-4000-8000-000000000004",
    title: "Журавль на облаке",
    created_at: "2026-09-11T21:00:00Z",
    updated_at: "2026-09-11T21:01:00Z",
  },
  {
    id: "20000000-0000-4000-8000-000000000001",
    title: "Подготовить макет",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:05:00Z",
  },
  {
    id: "20000000-0000-4000-8000-000000000002",
    title: "Идеи для проекта",
    created_at: "2026-01-02T00:00:00Z",
    updated_at: "2026-01-02T00:05:00Z",
  },
  {
    id: "20000000-0000-4000-8000-000000000003",
    title: "Тексты для сайта",
    created_at: "2026-01-03T00:00:00Z",
    updated_at: "2026-01-03T00:05:00Z",
  },
];

export const localWorkspacePreviewImageModels: ImageModelList = {
  items: [
    {
      id: "nano-banana-2",
      name: "Nano Banana 2",
      quality_options: ["1K", "2K", "4K"],
      price_by_quality: { "1K": 50, "2K": 70, "4K": 90 },
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 4,
      max_output_count: 4,
    },
    {
      id: "nano-banana-pro",
      name: "Nano Banana Pro",
      quality_options: ["1K", "2K", "4K"],
      price_by_quality: { "1K": 50, "2K": 80, "4K": 110 },
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 4,
      max_output_count: 4,
    },
    {
      id: "gpt-image-2",
      name: "GPT Image 2",
      quality_options: ["1K", "2K", "4K"],
      price_by_quality: { "1K": 40, "2K": 65, "4K": 90 },
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 4,
      max_output_count: 4,
    },
    {
      id: "seedream-4-5",
      name: "Seedream 4.5",
      quality_options: ["2K", "4K"],
      price_by_quality: { "2K": 30, "4K": 50 },
      default_quality: "2K",
      supports_reference_image: true,
      max_reference_images: 4,
      max_output_count: 4,
    },
    {
      id: "midjourney",
      name: "Midjourney",
      quality_options: ["1K", "2K"],
      price_by_quality: { "1K": 60, "2K": 85 },
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 4,
      max_output_count: 4,
    },
    {
      id: "flux-2-pro",
      name: "FLUX 2 Pro",
      quality_options: ["1K", "2K"],
      price_by_quality: { "1K": 45, "2K": 70 },
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 4,
      max_output_count: 4,
    },
  ],
};

export const localWorkspacePreviewImageJobs: ImageJobList = {
  items: [
    {
      id: "30000000-0000-4000-8000-000000000001",
      status: "succeeded",
      prompt: localPreviewCranePrompt,
      model_id: "nano-banana-2",
      model_name: "Nano Banana 2",
      image_quality: "2K",
      cost_estimate: 70,
      created_at: "2026-09-11T21:00:00Z",
      updated_at: "2026-09-11T21:01:00Z",
    },
    {
      id: "30000000-0000-4000-8000-000000000002",
      status: "succeeded",
      prompt: "Бумажный журавль над облаками",
      model_id: "gpt-image-2",
      model_name: "GPT Image 2",
      image_quality: "1K",
      cost_estimate: 40,
      created_at: "2026-08-29T10:00:00Z",
      updated_at: "2026-08-29T10:01:00Z",
    },
    {
      id: "30000000-0000-4000-8000-000000000003",
      status: "succeeded",
      prompt: "Абстрактные волны в мягком свете",
      model_id: "seedream-4-5",
      model_name: "Seedream 4.5",
      image_quality: "4K",
      cost_estimate: 50,
      created_at: "2026-08-28T08:00:00Z",
      updated_at: "2026-08-28T08:01:00Z",
    },
    {
      id: "30000000-0000-4000-8000-000000000004",
      status: "expired",
      prompt: "Минималистичный постер для кофейни",
      model_id: "nano-banana-pro",
      model_name: "Nano Banana Pro",
      image_quality: "2K",
      cost_estimate: 80,
      created_at: "2026-08-27T14:00:00Z",
      updated_at: "2026-08-27T14:30:00Z",
    },
    {
      id: "30000000-0000-4000-8000-000000000005",
      status: "awaiting_payment",
      prompt: "Редакционная съёмка продукта",
      model_id: "midjourney",
      model_name: "Midjourney",
      image_quality: "2K",
      cost_estimate: 85,
      created_at: "2026-08-26T09:00:00Z",
      updated_at: "2026-08-26T09:00:00Z",
    },
    {
      id: "30000000-0000-4000-8000-000000000006",
      status: "failed_terminal",
      prompt: "Концепт футуристичного интерфейса",
      model_id: "flux-2-pro",
      model_name: "FLUX 2 Pro",
      image_quality: "1K",
      cost_estimate: 45,
      created_at: "2026-08-25T16:00:00Z",
      updated_at: "2026-08-25T16:02:00Z",
    },
  ],
  has_more: false,
  next_cursor: null,
};

export const localWorkspacePreviewImageJobResults: Readonly<Record<string, ImageJobResult>> = {
  "30000000-0000-4000-8000-000000000001": {
    job_id: "30000000-0000-4000-8000-000000000001",
    status: "succeeded",
    artifacts: [
      {
        id: "40000000-0000-4000-8000-000000000001",
        mime_type: "image/png",
        size_bytes: 2358724,
        width: 1024,
        height: 1536,
      },
    ],
  },
  "30000000-0000-4000-8000-000000000002": {
    job_id: "30000000-0000-4000-8000-000000000002",
    status: "succeeded",
    artifacts: [
      {
        id: "40000000-0000-4000-8000-000000000002",
        mime_type: "image/png",
        size_bytes: 1024,
        width: 1024,
        height: 1024,
      },
    ],
  },
  "30000000-0000-4000-8000-000000000003": {
    job_id: "30000000-0000-4000-8000-000000000003",
    status: "succeeded",
    artifacts: [
      {
        id: "40000000-0000-4000-8000-000000000003",
        mime_type: "image/png",
        size_bytes: 1024,
        width: 1024,
        height: 1024,
      },
    ],
  },
};

export const localWorkspacePreviewImageArtifactPaths: Readonly<Record<string, string>> = {
  "40000000-0000-4000-8000-000000000001": "/assets/images/inspiration/paper-crane-cloud.png",
  "40000000-0000-4000-8000-000000000002": "/assets/images/workspace/neirohub-how-it-works-poster.png",
  "40000000-0000-4000-8000-000000000003": "/assets/images/models/default-model-87465de8.png",
};

export const localWorkspacePreviewConversationMessages: Readonly<Record<string, ConversationMessageList>> = {
  "20000000-0000-4000-8000-000000000004": {
    items: [
      {
        id: "24000000-0000-4000-8000-000000000001",
        seq: 1,
        role: "user",
        text: localPreviewCranePrompt,
        rating: null,
        created_at: "2026-09-11T21:00:00Z",
      },
      {
        id: "24000000-0000-4000-8000-000000000002",
        images: [{
          job: localWorkspacePreviewImageJobs.items[0],
          artifact: localWorkspacePreviewImageJobResults[localWorkspacePreviewImageJobs.items[0].id].artifacts[0],
        }],
        seq: 2,
        role: "assistant",
        text: "Готово — бумажный журавль на облаке в тёплом солнечном свете.\n\n![Белый бумажный журавль парит на пушистом облаке в золотистой комнате рядом с высоким арочным окном](/web/v1/image-artifacts/40000000-0000-4000-8000-000000000001)\n\n[Скачать изображение](/web/v1/image-artifacts/40000000-0000-4000-8000-000000000001)",
        rating: null,
        created_at: "2026-09-11T21:01:00Z",
      },
    ],
    has_more_before: false,
  },
  "20000000-0000-4000-8000-000000000001": {
    items: [
      {
        id: "21000000-0000-4000-8000-000000000001",
        seq: 1,
        role: "user",
        text: "Помоги подготовить структуру макета лендинга для нового AI-сервиса.",
        rating: null,
        created_at: "2026-01-01T00:01:00Z",
      },
      {
        id: "21000000-0000-4000-8000-000000000002",
        seq: 2,
        role: "assistant",
        text: "## Структура лендинга\n\n1. **Первый экран** — короткое обещание результата и основное действие.\n2. **Возможности** — три–четыре понятных сценария использования.\n3. **Как это работает** — путь пользователя от запроса до результата.\n4. **Примеры** — реальные материалы, созданные в сервисе.\n5. **Тариф и призыв к действию** — цена, ограничения и кнопка запуска.",
        rating: "like",
        created_at: "2026-01-01T00:02:00Z",
      },
      {
        id: "21000000-0000-4000-8000-000000000003",
        seq: 3,
        role: "user",
        text: "Сделай первый экран спокойным и без перегруженного маркетингового текста.",
        rating: null,
        created_at: "2026-01-01T00:03:00Z",
      },
      {
        id: "21000000-0000-4000-8000-000000000004",
        seq: 4,
        role: "assistant",
        text: "Можно оставить один крупный заголовок, короткое пояснение в две строки и поле для первого запроса. Остальные возможности лучше показать ниже — так пользователь сразу понимает главное действие и не отвлекается на детали.",
        rating: null,
        created_at: "2026-01-01T00:04:00Z",
      },
    ],
    has_more_before: false,
  },
  "20000000-0000-4000-8000-000000000002": {
    items: [
      {
        id: "22000000-0000-4000-8000-000000000001",
        seq: 1,
        role: "user",
        text: "Предложи несколько направлений для небольшого творческого проекта на выходные.",
        rating: null,
        created_at: "2026-01-02T00:01:00Z",
      },
      {
        id: "22000000-0000-4000-8000-000000000002",
        seq: 2,
        role: "assistant",
        text: "Вот три направления:\n\n- **Визуальный дневник района.** Собрать десять деталей городской среды и оформить их в единую серию.\n- **Мини-журнал о любимом занятии.** Сделать обложку, короткое интервью и одну полезную инструкцию.\n- **Экспериментальный ролик.** Соединить пять коротких сцен общей палитрой, ритмом и звуковой темой.\n\nКаждый вариант можно закончить за два дня и получить цельный результат, а не только набор черновиков.",
        rating: null,
        created_at: "2026-01-02T00:02:00Z",
      },
      {
        id: "22000000-0000-4000-8000-000000000003",
        seq: 3,
        role: "user",
        text: "Сравни их по сложности и времени.",
        rating: null,
        created_at: "2026-01-02T00:03:00Z",
      },
      {
        id: "22000000-0000-4000-8000-000000000004",
        seq: 4,
        role: "assistant",
        text: "### Сравнение\n\n- **Визуальный дневник:** низкая сложность, `4–6 часов`.\n- **Мини-журнал:** средняя сложность, `8–10 часов`.\n- **Экспериментальный ролик:** высокая сложность, `12–16 часов`.\n\nДля быстрого результата я бы выбрал **визуальный дневник**.",
        rating: null,
        created_at: "2026-01-02T00:04:00Z",
      },
    ],
    has_more_before: false,
  },
  "20000000-0000-4000-8000-000000000003": {
    items: [
      {
        id: "23000000-0000-4000-8000-000000000001",
        seq: 1,
        role: "user",
        text: "Напиши нейтральный текст для страницы сервиса, который помогает работать с нейросетями.",
        rating: null,
        created_at: "2026-01-03T00:01:00Z",
      },
      {
        id: "23000000-0000-4000-8000-000000000002",
        seq: 2,
        role: "assistant",
        text: "# Все нужные нейросети в одном месте\n\nСоздавайте тексты, изображения и другие материалы в едином рабочем пространстве. Выберите подходящую модель, опишите задачу и получите результат без сложной настройки.\n\n> Начните с простого запроса — параметры можно уточнить в процессе.\n\nКнопка действия: **Попробовать бесплатно**.",
        rating: null,
        created_at: "2026-01-03T00:02:00Z",
      },
      {
        id: "23000000-0000-4000-8000-000000000003",
        seq: 3,
        role: "user",
        text: "Покажи ещё короткий вариант для карточки возможности.",
        rating: null,
        created_at: "2026-01-03T00:03:00Z",
      },
      {
        id: "23000000-0000-4000-8000-000000000004",
        seq: 4,
        role: "assistant",
        text: "**Работайте с текстом**\n\nЗадавайте вопросы, развивайте идеи и сохраняйте весь диалог в одном месте.",
        rating: null,
        created_at: "2026-01-03T00:04:00Z",
      },
    ],
    has_more_before: false,
  },
};

export function isLocalWorkspacePreviewEnabled(): boolean {
  return (
    process.env.NODE_ENV === "development" &&
    process.env.NEIROHUB_LOCAL_WORKSPACE_PREVIEW === "1"
  );
}
