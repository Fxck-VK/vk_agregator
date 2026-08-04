import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

import HomePage, { metadata } from "./page";

describe("HomePage", () => {
  it("server-renders the cacheable public shell without a request nonce", () => {
    const markup = renderToStaticMarkup(<HomePage />);

    expect(markup).toContain("Простой старт в мир нейросетей");
    expect(markup).toContain('type="application/ld+json"');
    expect(markup).not.toContain("nonce=");
  });

  it("publishes complete indexable metadata for the public homepage", () => {
    expect(metadata.title).toBe("Нейросети онлайн на русском — NeiroHub");
    expect(metadata.description).toContain("нейросет");
    expect(metadata.alternates).toEqual({ canonical: "/" });
    expect(metadata.robots).toMatchObject({ index: true, follow: true });
    expect(metadata.openGraph).toMatchObject({
      type: "website",
      url: "/",
    });
    expect(metadata.twitter).toMatchObject({ card: "summary_large_image" });
  });
});
