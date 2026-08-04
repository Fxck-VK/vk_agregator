import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/server", () => ({
  connection: vi.fn(),
}));

vi.mock("next/headers", () => ({
  headers: vi.fn(),
}));

import { connection } from "next/server";
import { headers } from "next/headers";

import HomePage, { metadata } from "./page";

describe("HomePage", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("waits for the request so Next can attach the per-request CSP nonce", async () => {
    vi.mocked(connection).mockResolvedValue(undefined);
    vi.mocked(headers).mockResolvedValue(new Headers({ "x-nonce": "homepage-nonce" }) as never);

    const markup = renderToStaticMarkup(await HomePage());

    expect(connection).toHaveBeenCalledOnce();
    expect(markup).toContain("Простой старт в мир нейросетей");
    expect(markup).toContain('type="application/ld+json"');
    expect(markup).toContain('nonce="homepage-nonce"');
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
