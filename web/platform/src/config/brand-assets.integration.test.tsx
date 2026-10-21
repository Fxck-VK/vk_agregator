import { renderToStaticMarkup } from "react-dom/server";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const replacement = vi.hoisted(() => ({
  logo: "/assets/brand/marks/test-brand.png",
  icons: {
    icon: [{ url: "/assets/brand/favicons/test-icon-32.png", sizes: "32x32", type: "image/png" }],
    shortcut: "/assets/brand/favicons/test-icon-32.png",
    apple: [{ url: "/assets/brand/favicons/test-apple-180.png", sizes: "180x180", type: "image/png" }],
  },
}));

vi.mock("server-only", () => ({}));
vi.mock("@/config/brand", async importOriginal => ({
  ...await importOriginal<typeof import("./brand")>(),
  brandLogoPath: replacement.logo,
  brandIcons: replacement.icons,
}));
vi.mock("next/headers", () => ({
  headers: async () => new Headers(),
  cookies: async () => ({ get: () => undefined }),
}));
vi.mock("next/font/google", () => ({ Geist: () => ({ variable: "test-font" }) }));
vi.mock("next/navigation", () => ({
  usePathname: () => "/ru/app",
  useSearchParams: () => new URLSearchParams(),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
}));

import { generateMetadata } from "@/app/layout";
import { Sidebar } from "@/components/layout/Sidebar/Sidebar";
import { PublicHeader } from "@/components/public/PublicHeader/PublicHeader";
import { PublicFooter } from "@/components/public/PublicFooter/PublicFooter";
import { WorkspaceLanding } from "@/features/workspace/WorkspaceLanding/WorkspaceLanding";
import { getModelIconPath } from "@/features/models/model-icon-registry";
import { InspirationExampleCard } from "@/features/inspiration/InspirationExampleCard/InspirationExampleCard";
import { getInspirationExamples } from "@/features/inspiration/inspiration-examples";
import { publicDictionaryRu } from "@/i18n/public/ru";

describe("central brand artwork", () => {
  afterEach(cleanup);
  it.each([
    ["sidebar", <Sidebar key="sidebar" />],
    ["public header", <PublicHeader dictionary={publicDictionaryRu} key="header" />],
    ["public footer", <PublicFooter dictionary={publicDictionaryRu} key="footer" />],
    ["workspace footer", <WorkspaceLanding key="workspace" />],
  ])("applies a replacement logo from configuration to the %s", (_surface, element) => {
    const document = new DOMParser().parseFromString(renderToStaticMarkup(element), "text/html");
    const logos = [...document.querySelectorAll("img[data-brand-logo]")];
    expect(logos.length).toBeGreaterThan(0);
    for (const logo of logos) {
      expect(decodeURIComponent(logo.getAttribute("src") ?? "")).toContain(replacement.logo);
    }
    expect(document.querySelector('img[src*="neirohub-chip.png"]')).toBeNull();
  });

  it("uses the configured logo for the site's branded chat", () => {
    expect(getModelIconPath("chatgpt")).toBe(replacement.logo);
    expect(getModelIconPath("gpt_5")).not.toBe(replacement.logo);
  });

  it("applies a replacement logo to a branded example card and its preview", () => {
    const example = getInspirationExamples().find(item => !item.canRecreate)!;
    render(<InspirationExampleCard example={example} />);
    const card = screen.getByRole("button", { name: example.openLabel });
    const expectConfiguredLogo = (container: HTMLElement) => {
      const logo = container.querySelector("img[data-brand-logo]");
      expect(logo).not.toBeNull();
      expect(decodeURIComponent(logo!.getAttribute("src") ?? "")).toContain(replacement.logo);
      expect(within(container).queryByTestId("model-icon-fallback")).not.toBeInTheDocument();
    };
    expectConfiguredLogo(card);
    fireEvent.click(card);
    expectConfiguredLogo(screen.getByRole("dialog"));
  });

  it("applies replacement browser and device icons from the same configuration", async () => {
    expect((await generateMetadata()).icons).toEqual(replacement.icons);
  });
});
