import type { PublicDictionary } from "./dictionary";

export const publicDictionaryEn = {
  brand: {
    name: "NeiroHub",
  },
  navigation: {
    home: "Home",
    models: "AI models",
    tools: "Tools",
    articles: "Articles",
    prompts: "Prompts",
    guides: "Guides",
    pricing: "Pricing",
    openWorkspace: "Open platform",
  },
  contentKinds: {
    model: "AI model",
    tool: "Tool",
    article: "Article",
  },
  publication: {
    draft: "Draft",
    review: "In review",
    published: "Published",
    archived: "Archived",
  },
  common: {
    readMore: "Read more",
    viewAll: "View all",
    relatedContent: "Related content",
    updatedAt: "Updated",
    noResults: "No results",
    notFound: "Content not found",
  },
  theme: {
    label: "Appearance",
    system: "System theme",
    light: "Light theme",
    dark: "Dark theme",
  },
  footer: {
    tagline: "AI models in one workspace",
    workspace: "Workspace",
  },
  accessibility: {
    primaryNavigation: "Primary navigation",
    languageSwitcher: "Language selector",
    currentLanguage: "Current language",
    skipToContent: "Skip to content",
  },
} as const satisfies PublicDictionary;
