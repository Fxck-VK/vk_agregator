import { act, fireEvent, render, renderHook, screen } from "@testing-library/react";
import { useState, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
vi.mock("./actions", () => ({ setLocalePreference: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
import { setLocalePreference } from "./actions";
import { LanguageSwitcher } from "./LanguageSwitcher";
import { LocaleProvider } from "./LocaleProvider";
import type { Locale } from "./locales";
import { getDictionary } from "./dictionary";
import { RichMessage } from "./RichMessage";
import { FileTypeTabs } from "@/features/files/FileTypeTabs/FileTypeTabs";
import { ConversationComposer } from "@/features/conversations/ConversationComposer/ConversationComposer";
import { ModelCard } from "@/features/models/ModelCard/ModelCard";
import { useChatAttachments } from "@/features/conversations/use-chat-attachments";
import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";

describe("locale changes", () => {
  it("updates real controls and catalog descriptions without replacing the composer or translating its draft", () => {
    const child = <>
      <ConversationComposer contentVersion="1" forceScrollRequest={0} scrollContainer={null} onSubmit={vi.fn()} />
      <FileTypeTabs value="reports" onValueChange={vi.fn()} />
      <ModelCard model={{ id: "image", name: "Model name", description: "Создание изображений по текстовому описанию." }} />
    </>;
    const { rerender } = render(<LocaleProvider locale="ru">{child}</LocaleProvider>);
    const input = screen.getByRole("textbox");
    fireEvent.change(input, { target: { value: "Мой черновик {name} <script>" } });
    rerender(<LocaleProvider locale="en">{child}</LocaleProvider>);
    expect(screen.getByRole("textbox", { name: getDictionary("en").conversations.composerLabel })).toBe(input);
    expect(input).toHaveValue("Мой черновик {name} <script>");
    expect(screen.getByRole("tab", { name: "Reports" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText("Create images from a text description.")).toBeVisible();
    expect(screen.getByText("Model name")).toBeVisible();
  });
  it("translates an existing attachment validation error without losing its state", () => {
    let setLocale: (locale: Locale) => void;
    function Wrapper({ children }: { children: ReactNode }) {
      const [locale, updateLocale] = useState<Locale>("ru"); setLocale = updateLocale;
      return <LocaleProvider locale={locale}>{children}</LocaleProvider>;
    }
    const { result } = renderHook(() => useChatAttachments(), { wrapper: Wrapper });
    act(() => result.current.add([{ id: "file", name: "photo.png", mimeType: "image/png", source: "uploaded", file: new File(["image"], "photo.png", { type: "image/png" }) }]));
    expect(result.current.error).toMatch(/модель/);
    act(() => setLocale("en"));
    expect(result.current.error).toMatch(/selected model/i);
    expect(result.current.error).not.toMatch(/[А-Яа-я]/);
  });
  it("renders named rich-text slots with spacing and escaped content", () => {
    render(<LocaleProvider locale="en"><h1><RichMessage id="workspaceLanding.anEasyStartWith" values={{ highlight: <em>{"<AI>"}</em> }} /></h1></LocaleProvider>);
    expect(screen.getByRole("heading")).toHaveTextContent("An easy start with <AI>");
    expect(screen.getByRole("heading").querySelector("em")).toHaveTextContent("<AI>");
  });
  it("reports a failed language change and keeps the selected language", async () => {
    vi.mocked(setLocalePreference).mockRejectedValueOnce(new Error("Network unavailable"));
    render(<LocaleProvider locale="ru"><LanguageSwitcher /></LocaleProvider>);
    fireEvent.click(screen.getByRole("button", { name: "English" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(getDictionary("ru").account.languageFailure);
    expect(screen.getByRole("button", { name: "Русский" })).toHaveAttribute("aria-pressed", "true");
    expect(setLocalePreference).toHaveBeenCalledWith("en");
  });
  it("reverses horizontal keyboard navigation for RTL while preserving Home/End", () => {
    const readStyle = window.getComputedStyle;
    const spy = vi.spyOn(window, "getComputedStyle").mockImplementation(element => {
      const style = readStyle(element);
      Object.defineProperty(style, "direction", { value: "rtl" });
      return style;
    });
    try {
      const change = vi.fn();
      render(<ModeSwitchPanel activeID="b" ariaLabel="Modes" items={[{ id: "a", label: "A" }, { id: "b", label: "B" }, { id: "c", label: "C" }]} onChange={change} />);
      fireEvent.keyDown(screen.getByRole("button", { name: "B" }), { key: "ArrowRight" });
      expect(change).toHaveBeenLastCalledWith("a");
      fireEvent.keyDown(screen.getByRole("button", { name: "B" }), { key: "ArrowLeft" });
      expect(change).toHaveBeenLastCalledWith("c");
      fireEvent.keyDown(screen.getByRole("button", { name: "B" }), { key: "Home" });
      expect(change).toHaveBeenLastCalledWith("a");
    } finally { spy.mockRestore(); }
  });
});
