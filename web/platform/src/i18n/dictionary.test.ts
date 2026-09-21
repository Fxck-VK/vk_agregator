import { describe, expect, it } from "vitest";
import { getDictionary } from "./dictionary";
import { messageCatalogs, getTranslator } from "./messages";
import { locales } from "./locales";
import { localizedText, translateCatalogText } from "./catalog";
import { countLabel } from "./counts";
import { getCreditAmountLabel } from "@/components/ui/CreditAmount/CreditAmount";
import previewCatalog from "@/features/session/model-catalog.preview.json";

function leaves(value: unknown, path = ""): Record<string, string> {
  if (typeof value === "string") return { [path]: value };
  if (typeof value === "function") return { [path]: value("{argument}") };
  return Object.assign({}, ...Object.entries(value as object).map(([key, child]) => leaves(child, `${path}.${key}`)));
}
const parameters = (text: string) => [...new Set([...text.matchAll(/\{(\w+)\}/g)].map(match => match[1]))].sort();

describe("translation catalogs", () => {
  for (const locale of locales) {
    it(`keeps ${locale} keys, identifiers and message parameters complete`, () => {
      for (const [base, candidate] of [[leaves(getDictionary("ru")), leaves(getDictionary(locale))], [messageCatalogs.ru, messageCatalogs[locale]]]) {
        expect(Object.keys(candidate).sort()).toEqual(Object.keys(base).sort());
        for (const [key, text] of Object.entries(base)) {
          const translated = candidate[key as keyof typeof candidate];
          expect(translated, key).not.toBe("");
          expect(parameters(translated), key).toEqual(parameters(text));
          if (key.endsWith(".id")) expect(translated, key).toBe(text);
        }
      }
    });
  }
  it("formats counts by the selected language, including 0, 21 and fractions", () => {
    const ru = getTranslator("ru");
    const en = getTranslator("en");
    expect([0, 1, 2, 5, 21, 1.5].map(n => countLabel(ru, "files", n)))
      .toEqual(["0 файлов", "1 файл", "2 файла", "5 файлов", "21 файл", "1,5 файла"]);
    expect([0, 1, 2, 21].map(n => countLabel(en, "tokens", n)))
      .toEqual(["0 tokens", "1 token", "2 tokens", "21 tokens"]);
    expect(getCreditAmountLabel(0, undefined, ru)).toBe("0 звёзд");
    expect(getCreditAmountLabel(21, undefined, en)).toBe("21 stars");
    expect(getCreditAmountLabel(1.5, undefined, ru)).toBe("1,5 звезды");
  });
  it("resolves catalog translations without touching arbitrary or user-authored text", () => {
    expect(translateCatalogText(messageCatalogs.ru["catalog.imageDescription"], getTranslator("en")))
      .toBe(messageCatalogs.en["catalog.imageDescription"]);
    expect(translateCatalogText("Создание видео по текстовому описанию. Разрешения: 720p, 1080p.", getTranslator("en")))
      .toBe("Create videos from a text description. Resolutions: 720p, 1080p.");
    expect(translateCatalogText("Мой собственный текст", getTranslator("en"))).toBe("Мой собственный текст");
    expect(localizedText({ ru: "Описание", en: "Description" }, "en")).toBe("Description");
    expect(localizedText({ ru: "Описание" }, "en")).toBe("Описание");
  });
  it("covers the current server catalog descriptions without translating model names", () => {
    for (const model of previewCatalog.items) {
      expect(translateCatalogText(model.description, getTranslator("en")), model.id).not.toMatch(/[А-Яа-яЁё]/);
    }
  });
});
