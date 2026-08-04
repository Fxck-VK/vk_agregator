import type {
  LandingCapability,
  LandingFaqItem,
  LandingFooterGroup,
  LandingModel,
  LandingNewsItem,
  LandingTool,
} from "./landing-contracts";

const inspirationImage = "/inspiration/paper-crane-cloud.png";

export const landingTools: LandingTool[] = [
  {
    id: "neirohub-chat",
    name: "NeiroHub Chat",
    description: "Ответы, идеи и работа с текстом",
    href: "/login?next=/app/chats",
    icon: "✦",
    kind: "chat",
  },
  {
    id: "image-generator",
    name: "Генератор изображений",
    description: "Создание изображений по описанию",
    href: "/login?next=/app/image",
    icon: "◈",
    kind: "image",
  },
  {
    id: "gpt-image",
    name: "GPT Image",
    description: "Детальные иллюстрации и редактирование",
    href: "/login?next=/app/image",
    icon: "◎",
    kind: "image",
  },
  {
    id: "video-generator",
    name: "Генератор видео",
    description: "Будущий инструмент для коротких роликов",
    href: "/login?next=/app/models",
    icon: "▶",
    kind: "video",
  },
  {
    id: "text-models",
    name: "Текстовые модели",
    description: "Анализ, обучение и рабочие задачи",
    href: "/login?next=/app/models",
    icon: "Aa",
    kind: "text",
  },
  {
    id: "music-tools",
    name: "Музыка и аудио",
    description: "Инструменты для звука и музыки",
    href: "/login?next=/app/models",
    icon: "♫",
    kind: "music",
  },
  {
    id: "all-models",
    name: "90+ нейросетей",
    description: "Весь каталог в одном месте",
    href: "/login?next=/app/models",
    icon: "90+",
    kind: "catalog",
  },
];

export const landingNews: LandingNewsItem[] = [
  {
    id: "public-platform-preview",
    title: "NeiroHub становится полноценной веб-платформой",
    description: "Единое пространство объединит чаты, генерацию изображений, файлы и каталог моделей.",
    href: "/login?next=/app",
    imageSrc: inspirationImage,
    imageAlt: "Бумажный журавль над облаком в тёплой комнате — пример работы NeiroHub",
    linkLabel: "Открыть платформу",
  },
  {
    id: "image-workflow-preview",
    title: "Генерация изображений с прозрачной ценой",
    description: "Выберите модель и качество, сразу увидьте стоимость и следите за результатом в своих файлах.",
    href: "/login?next=/app/image",
    imageSrc: inspirationImage,
    imageAlt: "Пример изображения из галереи, созданного нейросетью по текстовому запросу",
    linkLabel: "Создать изображение",
  },
];

export const landingModels: LandingModel[] = [
  { id: "neirohub", name: "NeiroHub", description: "Универсальный бесплатный помощник", href: "/login?next=/app/chats", icon: "NH" },
  { id: "gpt-image-2", name: "GPT Image 2", description: "Генерация и редактирование изображений", href: "/login?next=/app/image", icon: "GI" },
  { id: "nano-banana", name: "Nano Banana", description: "Быстрые изображения и работа с референсами", href: "/login?next=/app/image", icon: "NB" },
  { id: "deepseek", name: "DeepSeek", description: "Рассуждения, анализ и тексты", href: "/login?next=/app/models", icon: "DS" },
  { id: "claude", name: "Claude", description: "Работа с документами и сложными задачами", href: "/login?next=/app/models", icon: "CL" },
  { id: "gemini", name: "Gemini", description: "Мультимодальные задачи и исследования", href: "/login?next=/app/models", icon: "GE" },
  { id: "video", name: "Видео-модели", description: "Создание роликов по тексту и изображениям", href: "/login?next=/app/models", icon: "VI" },
  { id: "audio", name: "Аудио-модели", description: "Музыка, речь и обработка звука", href: "/login?next=/app/models", icon: "AU" },
  { id: "presentations", name: "Презентации", description: "Структура и оформление слайдов", href: "/login?next=/app/models", icon: "PR" },
  { id: "study", name: "Учёба и работа", description: "Конспекты, объяснения и рабочие материалы", href: "/login?next=/app/models", icon: "ST" },
];

export const landingCapabilities: LandingCapability[] = [
  {
    id: "images",
    title: "Создавайте изображения",
    description: "Выбирайте модель, качество и референсы в одном понятном генераторе.",
    href: "/login?next=/app/image",
    imageSrc: inspirationImage,
    imageAlt: "Светлая сюрреалистичная композиция как пример генерации изображений в NeiroHub",
  },
  {
    id: "conversations",
    title: "Продолжайте диалоги",
    description: "История и контекст сохраняются в вашем рабочем пространстве.",
    href: "/login?next=/app/chats",
    imageSrc: inspirationImage,
    imageAlt: "Спокойная творческая сцена, сопровождающая возможности диалогов NeiroHub",
  },
  {
    id: "files",
    title: "Храните результаты",
    description: "Готовые изображения и будущие форматы собраны в библиотеке файлов.",
    href: "/login?next=/app/files",
    imageSrc: inspirationImage,
    imageAlt: "Пример результата нейросети, доступного в защищённой библиотеке файлов NeiroHub",
  },
  {
    id: "inspiration",
    title: "Используйте готовые идеи",
    description: "Изучайте примеры и переносите промпты в генератор одним действием.",
    href: "/login?next=/app/inspiration",
    imageSrc: inspirationImage,
    imageAlt: "Бумажный журавль на облаке как пример идеи и промпта из галереи NeiroHub",
  },
];

export const landingFaq: LandingFaqItem[] = [
  {
    id: "what-is-neirohub",
    question: "Что такое NeiroHub?",
    answer: "NeiroHub — единая веб-платформа для общения с нейросетями, генерации контента и хранения результатов.",
  },
  {
    id: "vpn",
    question: "Нужен ли VPN?",
    answer: "Нет. Работа с подключёнными инструментами выполняется через инфраструктуру NeiroHub.",
  },
  {
    id: "pricing",
    question: "Как рассчитывается стоимость?",
    answer: "Перед платным запуском интерфейс показывает стоимость в звёздах. Задача не запускается без явного подтверждения.",
  },
  {
    id: "files",
    question: "Где сохраняются результаты?",
    answer: "Подтверждённые сервером результаты появляются в разделе «Мои файлы» и доступны только владельцу аккаунта.",
  },
  {
    id: "models",
    question: "Можно ли выбирать конкретную модель?",
    answer: "Да. Пользователь выбирает инструмент явно; автоматическое перенаправление запроса между моделями не используется.",
  },
];

export const landingFooterGroups: LandingFooterGroup[] = [
  {
    id: "platform",
    title: "NeiroHub",
    links: [
      { label: "Рабочее пространство", href: "/login?next=/app" },
      { label: "Новый чат", href: "/login?next=/app/chats" },
      { label: "Мои файлы", href: "/login?next=/app/files" },
    ],
  },
  {
    id: "tools",
    title: "Инструменты",
    links: [
      { label: "Генерация изображений", href: "/login?next=/app/image" },
      { label: "Все нейросети", href: "/login?next=/app/models" },
      { label: "Вдохновение", href: "/login?next=/app/inspiration" },
    ],
  },
  {
    id: "start",
    title: "Начать работу",
    links: [
      { label: "Войти", href: "/login?next=/app" },
      { label: "Открыть главную", href: "/" },
    ],
  },
];
