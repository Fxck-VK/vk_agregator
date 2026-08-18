import { describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));

import {
  getArticleBySlug,
  getModelBySlug,
  getToolBySlug,
  listArticles,
  listModels,
  listTools,
} from "./content-repository";

describe("server-only content repository", () => {
  it("returns published localized models without raw multilingual records", () => {
    const items = listModels("en");

    expect(items).toHaveLength(1);
    expect(items[0]).toMatchObject({
      id: "model-gpt-image-2",
      kind: "model",
      locale: "en",
      slug: "gpt-image-2",
      title: "GPT Image 2",
      runtimeCatalogKey: "gpt-image-2",
    });
    expect(items[0]).not.toHaveProperty("translations");
    expect(items[0]).not.toHaveProperty("reviewer");
  });

  it("returns tools and articles in only the requested locale", () => {
    expect(listTools("ru")[0]).toMatchObject({
      locale: "ru",
      slug: "generator-izobrazheniy",
      workspacePath: "/app/image",
    });
    expect(listArticles("en")[0]).toMatchObject({
      locale: "en",
      slug: "image-prompt-basics",
      category: "guide",
    });
    expect(listArticles("en")[0]).not.toHaveProperty("translations");
  });

  it("finds current and legacy localized slugs and exposes the canonical slug", () => {
    expect(getModelBySlug("en", "openai-image-generator")).toMatchObject({
      id: "model-gpt-image-2",
      slug: "gpt-image-2",
    });
    expect(getToolBySlug("ru", "generator-izobrazheniy")).toMatchObject({
      id: "tool-image-generator",
      slug: "generator-izobrazheniy",
    });
    expect(getArticleBySlug("ru", "kak-napisat-promt-dlya-kartinki")).toMatchObject({
      id: "article-image-prompt-basics",
      slug: "osnovy-promtov-dlya-izobrazheniy",
    });
  });

  it("excludes drafts by default and includes them only with explicit server preview", () => {
    expect(listArticles("ru").map((article) => article.id)).not.toContain("article-content-workflow-draft");
    expect(
      listArticles("ru", { includeUnpublished: true }).map((article) => article.id),
    ).toContain("article-content-workflow-draft");
  });

  it("returns immutable, deterministically ordered result data", () => {
    const articles = listArticles("ru", { includeUnpublished: true });

    expect(articles.map((article) => article.title)).toEqual(
      [...articles.map((article) => article.title)].sort((left, right) => left.localeCompare(right, "ru")),
    );
    expect(Object.isFrozen(articles)).toBe(true);
    expect(Object.isFrozen(articles[0])).toBe(true);
    expect(Object.isFrozen(articles[0].body)).toBe(true);
  });

  it("returns undefined for a valid missing slug and rejects invalid input", () => {
    expect(getArticleBySlug("ru", "missing")).toBeUndefined();
    expect(() => getArticleBySlug("ru", "Not A Slug")).toThrow("Invalid content slug: Not A Slug");
    expect(() => listTools("de")).toThrow("Unsupported locale: de");
  });
});
