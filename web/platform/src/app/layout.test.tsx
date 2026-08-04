import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/script", () => ({
  default: ({ src, strategy }: { src: string; strategy: string }) => (
    <meta data-script-src={src} data-strategy={strategy} />
  ),
}));

import RootLayout, { metadata } from "./layout";

describe("RootLayout", () => {
  it("sets Russian as the document language", () => {
    const markup = renderToStaticMarkup(
      RootLayout({
        children: <main>Тест</main>,
      }),
    );
    const document = new DOMParser().parseFromString(markup, "text/html");

    expect(document.documentElement.getAttribute("lang")).toBe("ru");
  });

  it("defines the production metadata base for absolute social and canonical URLs", () => {
    expect(metadata.metadataBase?.toString()).toBe("https://neiirohub.ru/");
    expect(metadata.title).toEqual({
      default: "NeiroHub — нейросети на русском в одном месте",
      template: "%s | NeiroHub",
    });
  });

  it("loads the cacheable theme bootstrap before page content without request-bound data", () => {
    const markup = renderToStaticMarkup(
      RootLayout({
        children: <main>Theme content</main>,
      }),
    );
    const document = new DOMParser().parseFromString(markup, "text/html");
    const bootstrapScript = document.querySelector("head meta[data-script-src]");

    expect(document.documentElement.getAttribute("data-theme")).toBe("system");
    expect(bootstrapScript?.getAttribute("data-script-src")).toBe("/theme-bootstrap.js");
    expect(bootstrapScript?.getAttribute("data-strategy")).toBe("beforeInteractive");
    expect(bootstrapScript?.getAttribute("nonce")).toBeNull();
    expect(markup.indexOf("data-script-src")).toBeLessThan(markup.indexOf("<body"));
  });
});
