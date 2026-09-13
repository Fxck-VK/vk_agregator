import { assetPaths } from "@/assets/asset-paths";

export type WorkspaceHomeFaq = {
  answer: string;
  question: string;
};

export const capabilityLinks = [
  { href: "/app/chats", label: "Ответы на вопросы", icon: assetPaths.icons.features.answers },
  { href: "/app/image", label: "Генерация изображений", icon: assetPaths.icons.features.generateImage },
  { href: "/app/image", label: "Работа с референсами", icon: assetPaths.icons.features.references },
  { href: "/app/files", label: "Библиотека файлов", icon: assetPaths.icons.features.fileLibrary },
  { href: "/app/models", label: "Выбор нейросети", icon: assetPaths.icons.features.selectAi },
  { href: "/app/inspiration", label: "Идеи для промптов", icon: assetPaths.icons.features.promptIdeas },
] as const;

export const frequentlyAskedQuestions: WorkspaceHomeFaq[] = [
  {
    question: "Что такое NeiroHub?",
    answer:
      "NeiroHub — единое рабочее пространство для диалогов с нейросетями, генерации изображений и хранения результатов.",
  },
  {
    question: "Что такое собственные нейросети NeiroHub?",
    answer:
      "Так мы называем модели, доступные через единый интерфейс NeiroHub. Для каждой модели показаны её возможности, параметры и актуальная стоимость запуска.",
  },
  {
    question: "Что такое токены и подписка?",
    answer:
      "В NeiroHub звёзды используются как единицы баланса для запуска нейросетей. Отдельная подписка для базовой работы с платформой сейчас не требуется.",
  },
  {
    question: "Как купить подписку?",
    answer:
      "Сейчас подписка не продаётся. Для платных запусков достаточно пополнить баланс в профиле и выбрать подходящую модель.",
  },
  {
    question: "Есть ли бесплатный доступ?",
    answer:
      "Открывать рабочее пространство и изучать каталог можно бесплатно. Для запуска платных моделей потребуется достаточный баланс.",
  },
];
