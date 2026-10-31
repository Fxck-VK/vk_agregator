import { render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "@/i18n/LocaleProvider";
import { createLegalCatalog } from "@/features/legal/catalog.server";
import { legalSlugs } from "@/features/legal/model";

const request = vi.hoisted(() => ({ locale: "en" as "en" | "ru" }));
vi.mock("server-only", () => ({}));
vi.mock("@/i18n/server", async () => {
  const { getDictionary } = await import("@/i18n/dictionary");
  return {
    getRequestLocale: async () => request.locale,
    getRequestDictionary: async () => getDictionary(request.locale),
    notFound: () => { throw new Error("NOT_FOUND"); },
  };
});
import LegalIndex, { generateMetadata as indexMetadata } from "./page";
import LegalDocument, { generateMetadata } from "./[slug]/page";

beforeEach(() => {
  vi.stubEnv("LEGAL_OPERATOR_NAME", "Synthetic Test Operator");
  vi.stubEnv("LEGAL_OPERATOR_INN", "123456789012");
  vi.stubEnv("LEGAL_DOCUMENTS_PUBLISHED", "true");
});
afterEach(() => { request.locale = "en"; vi.unstubAllEnvs(); });

describe("legal reference pages", () => {
  it.each(legalSlugs)("preserves every reviewed paragraph and table in %s", async slug => {
    const catalog = createLegalCatalog("Synthetic Test Operator", "123456789012", true);
    const expected = catalog.documents.find(doc => doc.slug === slug)!;
    const { container } = render(<LocaleProvider locale="en">{await LegalDocument({ params: Promise.resolve({ slug }) })}</LocaleProvider>);
    const article = screen.getByRole("article");
    expect(article).toHaveAttribute("lang", "ru");
    const sections = article.querySelectorAll("section");
    expect(sections).toHaveLength(expected.sections.length);
    expected.sections.forEach((section, index) => {
      expect(sections[index].querySelector("h2")).toHaveTextContent(section.title);
      expect(Array.from(sections[index].querySelectorAll("p"), p => p.textContent)).toEqual(section.paragraphs);
      expect(Array.from(sections[index].querySelectorAll("th,td"), cell => cell.textContent)).toEqual(section.table.flat());
    });
    expect(container.querySelectorAll("[data-legal-page]")).toHaveLength(1);
    expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
    expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "Legal pages" })).toBeInTheDocument();
    expect(within(screen.getByRole("navigation", { name: "Legal pages" })).getByRole("link", { current: "page" })).toHaveAttribute("href", `/ru/legal/${slug}`);
    expect(screen.getByText("The authoritative legal text is provided in Russian.")).toBeInTheDocument();
  });

  it.each(["ru", "en"] as const)("retains localized links and noindex on %s", async locale => {
    request.locale = locale;
    render(<LocaleProvider locale={locale}>{await LegalIndex()}</LocaleProvider>);
    const main = screen.getByRole("main");
    legalSlugs.forEach(slug => expect(within(main).getAllByRole("link").some(link => link.getAttribute("href") === `/ru/legal/${slug}`)).toBe(true));
    expect((await generateMetadata({ params: Promise.resolve({ slug: "privacy-policy" }) })).robots).toEqual({ index: false, follow: true });
    expect((await indexMetadata()).robots).toEqual({ index: false, follow: true });
  });

  it("keeps unconfigured drafts fail-closed and hides unresolved operator fields", async () => {
    vi.stubEnv("LEGAL_OPERATOR_NAME", "");
    vi.stubEnv("LEGAL_OPERATOR_INN", "");
    const { container } = render(<LocaleProvider locale="en">{await LegalDocument({ params: Promise.resolve({ slug: "privacy-policy" }) })}</LocaleProvider>);
    expect(screen.getByText("Operator details have not been configured. This document cannot be accepted yet.")).toBeInTheDocument();
    expect(screen.getByText("Draft document. Not in effect yet.")).toBeInTheDocument();
    expect(container.querySelector("article section")).toBeNull();
    expect(container.textContent).not.toMatch(/\{NAME\}|\{INN\}/);
  });

  it("preserves 404 for unknown documents", async () => {
    await expect(LegalDocument({ params: Promise.resolve({ slug: "not-a-policy" }) })).rejects.toThrow("NOT_FOUND");
  });
});
