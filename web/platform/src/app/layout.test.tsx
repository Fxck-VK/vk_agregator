vi.mock("server-only", () => ({}));
import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/headers", () => ({
  headers: vi.fn(),
  cookies: vi.fn(async () => ({ get: () => undefined })),
}));

vi.mock("next/font/google", () => ({
  Geist: vi.fn(() => ({ variable: "font-geist-sans-test" })),
}));

import { headers, cookies } from "next/headers";
import { getDictionary } from "@/i18n/dictionary";

import RootLayout, { generateMetadata } from "./layout";

describe("RootLayout", () => {
  beforeEach(() => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-nonce": "test-theme-nonce" }) as never);
    vi.mocked(cookies).mockResolvedValue({ get: () => undefined } as never);
  });

  it("renders the saved locale and localized metadata on the server", async () => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-neirohub-page-locale": "en" }) as never);
    const markup = renderToStaticMarkup(await RootLayout({ children: <main>Content</main> }));
    const document = new DOMParser().parseFromString(markup, "text/html");
    expect(document.documentElement.lang).toBe("en");
    expect(document.documentElement.dir).toBe("ltr");
    expect(await generateMetadata()).toMatchObject({ title: getDictionary("en").document.title, description: getDictionary("en").document.description });
  });

  it("sets Russian as the document language", async () => {
    const markup = renderToStaticMarkup(
      await RootLayout({
        children: <main>Тест</main>,
      }),
    );
    const document = new DOMParser().parseFromString(markup, "text/html");

    expect(document.documentElement.getAttribute("lang")).toBe("ru");
  });

  it("loads Geist Sans as the global interface font", async () => {
    const markup = renderToStaticMarkup(
      await RootLayout({
        children: <main>Тест</main>,
      }),
    );
    const document = new DOMParser().parseFromString(markup, "text/html");

    expect(document.documentElement.classList.contains("font-geist-sans-test")).toBe(true);
  });

  it("uses dedicated square NeiroHub assets for browser and device icons", async () => {
    expect((await generateMetadata()).icons).toEqual({
      icon: [
        {
          url: "/assets/brand/favicons/neirohub-favicon-32.png",
          sizes: "32x32",
          type: "image/png",
        },
        {
          url: "/assets/brand/favicons/neirohub-favicon-48.png",
          sizes: "48x48",
          type: "image/png",
        },
      ],
      shortcut: "/assets/brand/favicons/neirohub-favicon-32.png",
      apple: [
        {
          url: "/assets/brand/favicons/neirohub-apple-touch-icon-180.png",
          sizes: "180x180",
          type: "image/png",
        },
      ],
    });
  });

  it("bootstraps the persisted theme in the head before page content with the request CSP nonce", async () => {
    const layout = await RootLayout({
      children: <main>Theme content</main>,
    });
    const markup = renderToStaticMarkup(layout);
    const document = new DOMParser().parseFromString(markup, "text/html");
    const bootstrapScript = document.querySelector("head script");

    expect(document.documentElement.getAttribute("data-theme")).toBe("system");
    expect(bootstrapScript?.textContent).toContain("neirohub.theme");
    expect(bootstrapScript?.getAttribute("nonce")).toBe("test-theme-nonce");
    expect(bootstrapScript?.getAttribute("type")).toBe("text/plain");
    expect(markup.indexOf("<script")).toBeLessThan(markup.indexOf("<body"));
  });
});
