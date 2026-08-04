export type WorkspaceHomeTool = {
  accent: "blue" | "cyan" | "violet" | "orange";
  description: string;
  href: string;
  label: string;
  monogram: string;
};

export type WorkspaceHomeFaq = {
  answer: string;
  question: string;
};

export const primaryTools: WorkspaceHomeTool[] = [
  {
    accent: "blue",
    description: "Обычный диалог с сохранением контекста",
    href: "/app/chats",
    label: "NeiroHub Chat",
    monogram: "NH",
  },
  {
    accent: "cyan",
    description: "Создание изображений по текстовому запросу",
    href: "/app/image",
    label: "Генератор изображений",
    monogram: "IMG",
  },
  {
    accent: "violet",
    description: "Модели для текста, изображений и других задач",
    href: "/app/models",
    label: "Каталог нейросетей",
    monogram: "AI",
  },
  {
    accent: "orange",
    description: "Готовые идеи и примеры запросов",
    href: "/app/inspiration",
    label: "Вдохновение",
    monogram: "IDEA",
  },
];

export const capabilityLinks = [
  { href: "/app/chats", label: "Ответы на вопросы" },
  { href: "/app/image", label: "Генерация изображений" },
  { href: "/app/image", label: "Работа с референсами" },
  { href: "/app/files", label: "Библиотека файлов" },
  { href: "/app/models", label: "Выбор нейросети" },
  { href: "/app/inspiration", label: "Идеи для промптов" },
] as const;

export const frequentlyAskedQuestions: WorkspaceHomeFaq[] = [
  {
    question: "Что такое NeiroHub?",
    answer:
      "NeiroHub — единое рабочее пространство для диалогов с нейросетями, генерации изображений и хранения результатов.",
  },
  {
    question: "Где сохраняются мои диалоги?",
    answer:
      "Диалоги текущего аккаунта появляются в боковой панели. Откройте любой из них, чтобы продолжить работу с сохранённым контекстом.",
  },
  {
    question: "Как рассчитывается стоимость генерации?",
    answer:
      "Актуальная стоимость показывается перед запуском задачи и зависит от выбранной модели и параметров. Списание происходит только через серверный контур.",
  },
  {
    question: "Где найти созданные изображения?",
    answer:
      "Готовые результаты и загруженные материалы доступны в разделе «Мои файлы» после подтверждения сервером.",
  },
  {
    question: "Можно ли пользоваться с телефона?",
    answer:
      "Да. Рабочая область, навигация и основные сценарии адаптированы для мобильных экранов.",
  },
];
