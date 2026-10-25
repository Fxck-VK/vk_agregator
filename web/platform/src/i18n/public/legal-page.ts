import type { LegalSlug } from "@/features/legal/model";

export type LegalNavigationItem = { readonly slug: LegalSlug; readonly title: string };
export const legalNavigationGroups = [
  { key: "terms", slugs: ["user-agreement"] },
  { key: "payments", slugs: ["paid-access", "payment-details"] },
  { key: "privacy", slugs: ["privacy-policy", "marketing-consent"] },
] as const;

export const legalPageText = {
  en: {
    title: "Legal pages", navigation: "Navigation", breadcrumb: "Breadcrumb",
    openNavigation: "Open navigation", openLegalMenu: "Open legal menu", closeMenu: "Close menu",
    skip: "Skip to document", home: "Home", newChat: "New chat", models: "All AI models", files: "My files",
    workspace: "Open platform", login: "Log in", signup: "Sign up", support: "Contact support",
    groups: { terms: "General Terms of Service", payments: "Payments and billing", privacy: "Privacy and Data" },
    help: "Help", intro: "The terms, policies and payment information for using Epifera, all in one place.",
    table: "Document table", tableHint: "Scroll horizontally to read the complete table.",
  },
  ru: {
    title: "Юридические документы", navigation: "Навигация", breadcrumb: "Навигационная цепочка",
    openNavigation: "Открыть навигацию", openLegalMenu: "Открыть меню документов", closeMenu: "Закрыть меню",
    skip: "Перейти к документу", home: "Главная", newChat: "Новый чат", models: "Все нейросети", files: "Мои файлы",
    workspace: "Открыть платформу", login: "Войти", signup: "Регистрация", support: "Написать в поддержку",
    groups: { terms: "Условия использования", payments: "Оплата и возвраты", privacy: "Конфиденциальность и данные" },
    help: "Помощь", intro: "Условия использования Epifera, политика обработки данных и информация об оплате — в одном месте.",
    table: "Таблица документа", tableHint: "Прокрутите по горизонтали, чтобы прочитать таблицу целиком.",
  },
} as const;
