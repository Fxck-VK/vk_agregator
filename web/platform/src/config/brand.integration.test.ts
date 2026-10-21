import { describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));
vi.mock("@/config/brand", async importOriginal => ({
  ...await importOriginal<typeof import("./brand")>(),
  brandName: "Пример & Co",
  brandFilePrefix: "пример-co",
  brandMonogram: "ПC",
}));

import { articles } from "@/content/data/articles";
import { models } from "@/content/data/models";
import { tools } from "@/content/data/tools";
import { getInspirationExamples } from "@/features/inspiration/inspiration-examples";
import { parseModelCatalog, projectChatModelCatalog } from "@/features/models/model-catalog-contract";
import catalogFixture from "@/features/session/model-catalog.preview.json";
import { getDictionary } from "@/i18n/dictionary";
import { getTranslator } from "@/i18n/messages";
import { publicDictionaryEn } from "@/i18n/public/en";
import { publicDictionaryRu } from "@/i18n/public/ru";

describe("website brand configuration", () => {
  it.each(["ru", "en"] as const)("uses the brand prefix in downloaded image and video examples in %s", locale => {
    const examples = getInspirationExamples(locale);
    expect(examples.find(example => example.id === "paper-crane-cloud")?.downloadName).toBe("пример-co-paper-crane-cloud.png");
    expect(examples.find(example => example.id === "spider-man-selfie")?.downloadName).toBe("пример-co-spider-man-selfie.png");
    for (const example of examples) {
      expect(example.downloadName).toMatch(/^пример-co-.+\.(png|mp4)$/);
      expect(example.downloadName.endsWith(".mp4")).toBe(example.mediaType === "video");
    }
  });

  it.each(["ru", "en"] as const)("uses one display name throughout the %s interface", locale => {
    const dictionary = getDictionary(locale);
    const msg = getTranslator(locale);
    const publicDictionary = locale === "ru" ? publicDictionaryRu : publicDictionaryEn;
    expect(dictionary.brand.name).toBe("Пример & Co");
    expect(dictionary.brand.monogram).toBe("ПC");
    expect(publicDictionary.brand.name).toBe("Пример & Co");
    const copy = [
      dictionary.document.title,
      dictionary.auth.title,
      dictionary.auth.registrationTitle,
      dictionary.profile.referralLaunchTitle,
      dictionary.profile.inviteTitle,
      dictionary.conversations.composerPlaceholder,
      dictionary.conversations.composerAwaitingResponse,
      dictionary.conversations.assistantRole,
      msg("workspaceHomeContent.whatIsNeirohub"),
      msg("workspaceLanding.howNeirohubWorks"),
      msg("workspaceLanding.2026NeirohubAllRightsReserved"),
      msg("workspacePrompt.askNeirohubOr"),
      msg("modelCardContent.valueIsAvailableInNeirohub", { value1: "GPT Image 2" }),
    ];
    for (const text of copy) {
      expect(text).toContain("Пример & Co");
      expect(text).not.toContain("NeiroHub");
    }
    expect(getInspirationExamples(locale).find(example => !example.canRecreate)?.modelName).toBe("Пример & Co");
  });

  it("uses the brand in public editorial copy while preserving permanent URLs", () => {
    expect(models[0].translations.ru.seo.title).toContain("Пример & Co");
    expect(models[0].translations.en.seo.description).toContain("Пример & Co");
    expect(tools[0].translations.ru.seo.title).toContain("Пример & Co");
    expect(tools[0].translations.en.seo.title).toContain("Пример & Co");
    expect(articles[0].author).toBe("Пример & Co editorial");
    expect(articles[1].translations.ru.title).toContain("Пример & Co");
    expect(articles[1].translations.ru.slug).toBe("redakcionnyj-process-neirohub");
  });

  it("relabels only the branded chat model without changing its ID, price or provider data", () => {
    const original = structuredClone(catalogFixture);
    const catalog = parseModelCatalog(catalogFixture);
    const chat = catalog.items.find(model => model.id === "chatgpt")!;
    const source = original.items.find(model => model.id === "chatgpt")!;
    expect(chat.name).toBe("Пример & Co Chat");
    expect({ ...chat, name: source.name }).toEqual(source);
    expect(projectChatModelCatalog(catalog).items.find(model => model.id === "chatgpt")?.name).toBe(chat.name);
    expect(catalog.items.filter(model => model.id !== "chatgpt")).toEqual(original.items.filter(model => model.id !== "chatgpt"));
    expect(catalogFixture).toEqual(original);
  });
});
