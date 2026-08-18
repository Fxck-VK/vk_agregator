export type PublicDictionary = {
  readonly brand: {
    readonly name: string;
  };
  readonly navigation: {
    readonly home: string;
    readonly models: string;
    readonly tools: string;
    readonly articles: string;
    readonly prompts: string;
    readonly guides: string;
    readonly pricing: string;
    readonly openWorkspace: string;
  };
  readonly contentKinds: {
    readonly model: string;
    readonly tool: string;
    readonly article: string;
  };
  readonly publication: {
    readonly draft: string;
    readonly review: string;
    readonly published: string;
    readonly archived: string;
  };
  readonly common: {
    readonly readMore: string;
    readonly viewAll: string;
    readonly relatedContent: string;
    readonly updatedAt: string;
    readonly noResults: string;
    readonly notFound: string;
  };
  readonly theme: {
    readonly label: string;
    readonly system: string;
    readonly light: string;
    readonly dark: string;
  };
  readonly footer: {
    readonly tagline: string;
    readonly workspace: string;
  };
  readonly accessibility: {
    readonly primaryNavigation: string;
    readonly languageSwitcher: string;
    readonly currentLanguage: string;
    readonly skipToContent: string;
  };
};
