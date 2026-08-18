import type { PublicDictionary } from "./dictionary";

export const publicDictionaryRu = {
  brand: {
    name: "NeiroHub",
  },
  navigation: {
    home: "Главная",
    models: "Нейросети",
    tools: "Инструменты",
    articles: "Статьи",
    prompts: "Промпты",
    guides: "Инструкции",
    pricing: "Тарифы",
    openWorkspace: "Открыть платформу",
  },
  contentKinds: {
    model: "Нейросеть",
    tool: "Инструмент",
    article: "Статья",
  },
  publication: {
    draft: "Черновик",
    review: "На проверке",
    published: "Опубликовано",
    archived: "В архиве",
  },
  common: {
    readMore: "Подробнее",
    viewAll: "Показать всё",
    relatedContent: "Материалы по теме",
    updatedAt: "Обновлено",
    noResults: "Ничего не найдено",
    notFound: "Материал не найден",
  },
  theme: {
    label: "Тема оформления",
    system: "Системная тема",
    light: "Светлая тема",
    dark: "Тёмная тема",
  },
  footer: {
    tagline: "Нейросети в одном рабочем пространстве",
    workspace: "Рабочее пространство",
  },
  accessibility: {
    primaryNavigation: "Основная навигация",
    languageSwitcher: "Выбор языка",
    currentLanguage: "Текущий язык",
    skipToContent: "Перейти к содержимому",
  },
} as const satisfies PublicDictionary;
