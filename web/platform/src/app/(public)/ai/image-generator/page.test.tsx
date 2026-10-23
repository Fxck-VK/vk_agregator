import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "@/i18n/LocaleProvider";
import ImageGeneratorExamplePage, { generateMetadata } from "./page";

vi.mock("server-only", () => ({}));
vi.mock("next/headers", () => ({ headers: vi.fn(async () => new Headers()), cookies: vi.fn(async () => ({ get: () => undefined })) }));
vi.mock("next/navigation", () => ({ useRouter: vi.fn() }));

describe("image tool page demonstration", () => {
  it("keeps the unfinished example outside search indexing", async () => {
    expect((await generateMetadata()).robots).toEqual({ index: false, follow: false });
  });

  it("changes the visual example without sending a generation request", async () => {
    const request = vi.spyOn(globalThis, "fetch");
    render(<LocaleProvider locale="ru">{await ImageGeneratorExamplePage()}</LocaleProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Кампания" }));
    expect(screen.getByRole("button", { name: "Кампания" })).toHaveAttribute("aria-pressed", "true");
    expect(request).not.toHaveBeenCalled();
    expect(screen.getAllByRole("link", { name: /Открыть генератор/ })[0]).toHaveAttribute("href", "/ru/app/image");
    request.mockRestore();
  });

  it("uses the English URL locale for content and workspace links", async () => {
    render(<LocaleProvider locale="en">{await ImageGeneratorExamplePage()}</LocaleProvider>);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("AI image generator");
    expect(screen.getAllByRole("link", { name: /Open the generator/ })[0]).toHaveAttribute("href", "/en/app/image");
  });
});
