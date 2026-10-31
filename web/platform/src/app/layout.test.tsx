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
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { brandIcons } from "@/config/brand";
import { getDictionary } from "@/i18n/dictionary";

import RootLayout, { generateMetadata } from "./layout";

describe("RootLayout", () => {
  beforeEach(() => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-nonce": "test-theme-nonce" }) as never);
    vi.mocked(cookies).mockResolvedValue({ get: () => undefined } as never);
  });

  it("renders Russian document metadata even with a retired English locale header", async () => {
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-neirohub-page-locale": "en" }) as never);
    const markup = renderToStaticMarkup(await RootLayout({ children: <main>Content</main> }));
    const document = new DOMParser().parseFromString(markup, "text/html");
    expect(document.documentElement.lang).toBe("ru");
    expect(document.documentElement.dir).toBe("ltr");
    expect(await generateMetadata()).toMatchObject({ title: getDictionary("ru").document.title, description: getDictionary("ru").document.description });
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

  it("uses configured browser icons whose files match their declared sizes", async () => {
    expect((await generateMetadata()).icons).toEqual(brandIcons);
    for (const icon of [...brandIcons.icon, ...brandIcons.apple]) {
      const file = readFileSync(resolve(process.cwd(), "public", icon.url.slice(1)));
      expect(file.byteLength).toBeGreaterThan(8);
      if (icon.type === "image/png") {
        const [width, height] = icon.sizes.split("x").map(Number);
        expect(file.subarray(0, 8).toString("hex")).toBe("89504e470d0a1a0a");
        expect(file.readUInt32BE(16)).toBe(width);
        expect(file.readUInt32BE(20)).toBe(height);
      }
    }
    expect(brandIcons.icon.map(icon => icon.url)).toContain(brandIcons.shortcut);
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
