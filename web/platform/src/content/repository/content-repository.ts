import "server-only";

import { rawContentGraph } from "../data/content";
import { contentSlugSchema } from "../domain/schemas";
import type {
  ArticleContent,
  ArticleTranslation,
  ModelContent,
  ModelTranslation,
  ToolContent,
  ToolTranslation,
} from "../domain/types";
import { parseLocale, type Locale } from "../../i18n/locales";
import { validateContentGraph } from "./content-validation";

type DeepReadonly<T> = T extends readonly (infer Item)[]
    ? readonly DeepReadonly<Item>[]
    : T extends object
      ? { readonly [Key in keyof T]: DeepReadonly<T[Key]> }
      : T;

type BaseTranslation = {
  slug: string;
  legacySlugs: string[];
  title: string;
};

type LocalizableEntity<Translation> = {
  status: ModelContent["status"];
  reviewer?: string;
  translations: Partial<Record<Locale, Translation>>;
};

type LocalizedMutable<Entity, Translation> = Omit<Entity, "translations" | "reviewer"> &
  Translation & { locale: Locale };

type LocalizedRecord<Entity, Translation> = DeepReadonly<LocalizedMutable<Entity, Translation>>;

export type LocalizedModelContent = LocalizedRecord<ModelContent, ModelTranslation>;
export type LocalizedToolContent = LocalizedRecord<ToolContent, ToolTranslation>;
export type LocalizedArticleContent = LocalizedRecord<ArticleContent, ArticleTranslation>;

export type ContentListOptions = {
  includeUnpublished?: boolean;
};

const contentGraph = validateContentGraph(rawContentGraph);

function deepFreeze<T>(value: T): DeepReadonly<T> {
  if (typeof value !== "object" || value === null || Object.isFrozen(value)) {
    return value as DeepReadonly<T>;
  }

  for (const child of Object.values(value)) {
    deepFreeze(child);
  }

  return Object.freeze(value) as DeepReadonly<T>;
}

function includeRecord(status: ModelContent["status"], options?: ContentListOptions): boolean {
  return status === "published" || options?.includeUnpublished === true;
}

function localizeRecord<
  Entity extends LocalizableEntity<Translation>,
  Translation extends BaseTranslation,
>(
  entity: Entity,
  locale: Locale,
): LocalizedMutable<Entity, Translation> | undefined {
  const translation = entity.translations[locale];
  if (!translation) return undefined;

  const clone = structuredClone(entity);
  const { translations, reviewer, ...safeEntity } = clone;
  void translations;
  void reviewer;

  return {
    ...safeEntity,
    ...structuredClone(translation),
    locale,
  } as LocalizedMutable<Entity, Translation>;
}

function parseSlug(input: unknown): string {
  const parsed = contentSlugSchema.safeParse(input);
  if (!parsed.success) {
    throw new Error(`Invalid content slug: ${String(input)}`);
  }
  return parsed.data;
}

function sortLocalized<T extends { title: string }>(items: T[], locale: Locale): readonly DeepReadonly<T>[] {
  items.sort((left, right) => left.title.localeCompare(right.title, locale));
  return deepFreeze(items);
}

function listLocalized<
  Entity extends LocalizableEntity<Translation>,
  Translation extends BaseTranslation,
>(entities: readonly Entity[], localeInput: unknown, options?: ContentListOptions) {
  const locale = parseLocale(localeInput);
  const localized = entities
    .filter((entity) => includeRecord(entity.status, options))
    .map((entity) => localizeRecord<Entity, Translation>(entity, locale))
    .filter((entity): entity is LocalizedMutable<Entity, Translation> => entity !== undefined);

  return sortLocalized(localized, locale);
}

function getLocalizedBySlug<
  Entity extends LocalizableEntity<Translation>,
  Translation extends BaseTranslation,
>(entities: readonly Entity[], localeInput: unknown, slugInput: unknown, options?: ContentListOptions) {
  const locale = parseLocale(localeInput);
  const slug = parseSlug(slugInput);
  const entity = entities.find((candidate) => {
    if (!includeRecord(candidate.status, options)) return false;
    const translation = candidate.translations[locale];
    return translation?.slug === slug || translation?.legacySlugs.includes(slug) === true;
  });

  const localized = entity ? localizeRecord<Entity, Translation>(entity, locale) : undefined;
  return localized ? deepFreeze(localized) : undefined;
}

export function listModels(locale: unknown, options?: ContentListOptions): readonly LocalizedModelContent[] {
  return listLocalized<ModelContent, ModelTranslation>(contentGraph.models, locale, options);
}

export function getModelBySlug(
  locale: unknown,
  slug: unknown,
  options?: ContentListOptions,
): LocalizedModelContent | undefined {
  return getLocalizedBySlug<ModelContent, ModelTranslation>(contentGraph.models, locale, slug, options);
}

export function listTools(locale: unknown, options?: ContentListOptions): readonly LocalizedToolContent[] {
  return listLocalized<ToolContent, ToolTranslation>(contentGraph.tools, locale, options);
}

export function getToolBySlug(
  locale: unknown,
  slug: unknown,
  options?: ContentListOptions,
): LocalizedToolContent | undefined {
  return getLocalizedBySlug<ToolContent, ToolTranslation>(contentGraph.tools, locale, slug, options);
}

export function listArticles(locale: unknown, options?: ContentListOptions): readonly LocalizedArticleContent[] {
  return listLocalized<ArticleContent, ArticleTranslation>(contentGraph.articles, locale, options);
}

export function getArticleBySlug(
  locale: unknown,
  slug: unknown,
  options?: ContentListOptions,
): LocalizedArticleContent | undefined {
  return getLocalizedBySlug<ArticleContent, ArticleTranslation>(contentGraph.articles, locale, slug, options);
}
