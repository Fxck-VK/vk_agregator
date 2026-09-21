/* eslint-disable @next/next/no-html-link-for-pages -- Native link fixtures and router mocks are intentional in these tests. */
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(),
  useSearchParams: vi.fn(() => new URLSearchParams()),
}));
vi.mock("@/features/models/generation-model-catalog", () => ({ loadGenerationModelCatalog: vi.fn() }));
vi.mock("@/lib/web-api/browser", () => ({ webBrowserMutation: vi.fn(), webBrowserFetch: vi.fn() }));

import { useRouter } from "next/navigation";
import { loadGenerationModelCatalog } from "@/features/models/generation-model-catalog";
import { WorkspaceConversationListProvider } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { readPendingConversationBootstrap } from "@/features/conversations/pending-conversation-bootstrap";
import { webBrowserMutation } from "@/lib/web-api/browser";
import { ru } from "@/i18n/ru";
import { imageModelFixture } from "@/test/model-catalog";
import { WorkspaceHero } from "./WorkspaceHero";

const push = vi.fn();
const requestId = "11111111-1111-4111-8111-111111111111";
const chatModel = {
  id: "neirohub-chat",
  name: "NeiroHub Chat",
  description: "Текстовая модель из общего каталога",
  category: "text" as const,
  categories: ["popular", "text", "study-work", "free"],
  estimate_credits: 0,
  isFree: true,
};
function imageGenerationModel(model: Parameters<typeof imageModelFixture>[0]) {
  return { ...imageModelFixture(model), category: "images" as const };
}
const catalogue = {
  default_model_id: chatModel.id,
  categoryErrors: {},
  items: [
    chatModel,
    imageGenerationModel({ id: "nano-banana-pro", name: "Nano Banana Pro", quality_options: ["2K", "4K"], default_quality: "2K", price_by_quality: { "2K": 30, "4K": 60 }, supports_reference_image: true, max_reference_images: 1, max_output_count: 4, categories: ["popular", "images"] }),
    imageGenerationModel({ id: "nano-banana-2", name: "Nano Banana 2", quality_options: ["1K"], default_quality: "1K", price_by_quality: { "1K": 10 }, supports_reference_image: false, max_reference_images: 0, max_output_count: 1, categories: ["popular", "images"] }),
  ],
} as Awaited<ReturnType<typeof loadGenerationModelCatalog>>;

function renderHero(access: "authenticated" | "guest" = "authenticated") {
  return render(
    <WorkspaceConversationListProvider accountId="hero-test" initialConversations={[]}>
      <WorkspaceHero access={access} modelLinksClassName="models" allModelsLink={<a href="/app/models">Все нейросети</a>} />
    </WorkspaceConversationListProvider>,
  );
}
const selectModel = async (name: string) => fireEvent.click(await screen.findByRole("button", { name: `Выбрать модель: ${name}` }));

beforeEach(() => {
  vi.mocked(useRouter).mockReturnValue({ push } as never);
  vi.mocked(loadGenerationModelCatalog).mockResolvedValue(catalogue);
  vi.stubGlobal("crypto", { randomUUID: vi.fn(() => requestId) });
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
  window.sessionStorage.clear();
});

describe("WorkspaceHero model selection", () => {
  it("submits the loaded default text model before an image shortcut is selected", async () => {
    renderHero();
    await screen.findByRole("button", { name: "Выбрать модель: NeiroHub Chat" });
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "Помоги с планом" } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));

    expect(push).toHaveBeenCalledExactlyOnceWith(`/ru/app/chat/${requestId}?pending=1`);
    expect(readPendingConversationBootstrap(requestId)).toMatchObject({
      modelId: chatModel.id,
      prompt: "Помоги с планом",
    });
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });

  it("keeps text submission disabled when the catalog has no text model", async () => {
    vi.mocked(loadGenerationModelCatalog).mockResolvedValue({
      ...catalogue,
      default_model_id: "",
      items: catalogue.items.filter((model) => model.category !== "text"),
    } as Awaited<ReturnType<typeof loadGenerationModelCatalog>>);
    renderHero();
    await screen.findByRole("button", { name: "Выбрать модель: Nano Banana Pro" });
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "Текст без модели" } });

    expect(screen.getByRole("button", { name: ru.workspace.promptSubmit })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));
    expect(push).not.toHaveBeenCalled();
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });

  it("keeps outgoing settings inert for the exit animation and then removes them", async () => {
    const { container } = renderHero();
    await selectModel("Nano Banana Pro");
    fireEvent.click(screen.getByRole("button", { name: "Разрешение: 2K" }));
    fireEvent.click(screen.getByRole("radio", { name: "4K" }));
    const input = screen.getByRole("textbox");
    vi.useFakeTimers();
    fireEvent.click(screen.getByRole("button", { name: "Выбрать модель: NeiroHub Chat" }));

    const outgoing = container.querySelector('[data-state="exiting"]');
    expect(outgoing).toHaveAttribute("inert");
    expect(outgoing).toHaveAttribute("aria-hidden", "true");
    expect(outgoing?.querySelector('[aria-label="Разрешение: 4K"]')).not.toBeNull();
    expect(screen.queryByRole("button", { name: /^Разрешение:/ })).toBeNull();
    expect(screen.getByRole("button", { name: ru.workspace.promptSubmit })).toBeInTheDocument();
    act(() => vi.advanceTimersByTime(219));
    expect(outgoing).toBeInTheDocument();
    act(() => vi.advanceTimersByTime(1));
    expect(outgoing).not.toBeInTheDocument();
    expect(screen.getByRole("textbox")).toBe(input);
  });

  it("cancels an old exit when another model is selected before the animation ends", async () => {
    const { container } = renderHero();
    await selectModel("Nano Banana Pro");
    vi.useFakeTimers();
    fireEvent.click(screen.getByRole("button", { name: "Выбрать модель: NeiroHub Chat" }));
    act(() => vi.advanceTimersByTime(100));
    fireEvent.click(screen.getByRole("button", { name: "Выбрать модель: Nano Banana 2" }));
    expect(container.querySelector('[data-state="exiting"]')).toBeNull();
    act(() => vi.advanceTimersByTime(120));
    expect(screen.getByRole("button", { name: "Соотношение сторон: 16:9" })).toBeVisible();
    expect(screen.queryByRole("button", { name: "Разрешение: 1K" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Выбрать модель: NeiroHub Chat" }));
    act(() => vi.advanceTimersByTime(219));
    expect(container.querySelector('[data-state="exiting"]')).not.toBeNull();
    act(() => vi.advanceTimersByTime(1));
    expect(container.querySelector('[data-state="exiting"]')).toBeNull();
  });

  it("removes settings immediately when reduced motion is requested", async () => {
    vi.stubGlobal("matchMedia", vi.fn().mockReturnValue({ matches: true }));
    const { container } = renderHero();
    await selectModel("Nano Banana Pro");
    await selectModel("NeiroHub Chat");
    expect(container.querySelector('[data-state="exiting"]')).toBeNull();
    expect(container.querySelector('[aria-label^="Разрешение:"]')).toBeNull();
  });

  it("keeps the same textarea, surface and caret after a rejected file and model changes", async () => {
    const { container } = renderHero();
    await selectModel("NeiroHub Chat");
    const input = screen.getByRole("textbox") as HTMLTextAreaElement;
    const surface = input.closest('[data-ui="input-surface"]');
    const form = input.closest("form");
    fireEvent.change(input, { target: { value: "Один постоянный инпут" } });
    input.focus();
    input.setSelectionRange(2, 8);
    const upload = container.querySelector('input[type="file"]')!;
    fireEvent.change(upload, { target: { files: [new File(["brief"], "brief.txt", { type: "text/plain" })] } });
    expect(screen.queryByRole("button", { name: "Убрать brief.txt" })).toBeNull();
    expect(webBrowserMutation).not.toHaveBeenCalled();

    for (const model of ["Nano Banana Pro", "Nano Banana 2", "NeiroHub Chat"]) {
      await selectModel(model);
      expect(screen.getByRole("textbox")).toBe(input);
      expect(input.closest('[data-ui="input-surface"]')).toBe(surface);
      expect(input.closest("form")).toBe(form);
      expect(input.selectionStart).toBe(2);
      expect(input.selectionEnd).toBe(8);
      expect(screen.queryByRole("button", { name: "Убрать brief.txt" })).toBeNull();
      expect(container.querySelectorAll('[data-ui="input-surface"]')).toHaveLength(1);
      expect(screen.queryAllByLabelText(/^Стоимость:/)).toHaveLength(model === "NeiroHub Chat" ? 0 : 1);
    }
  });

  it("changes the form in place and keeps the draft across image and chat modes", async () => {
    renderHero();
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "Мой журавль" } });
    await selectModel("Nano Banana Pro");
    expect(screen.getByRole("textbox", { name: "Задайте вопрос NeiroHub" })).toHaveValue("Мой журавль");
    expect(screen.getByRole("button", { name: "Выбрать модель: Nano Banana Pro" })).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(screen.getByRole("button", { name: "Разрешение: 2K" }));
    fireEvent.click(screen.getByRole("radio", { name: "4K" }));
    await selectModel("Nano Banana 2");
    expect(screen.getByRole("button", { name: "Соотношение сторон: 16:9" })).toBeVisible();
    expect(screen.queryByRole("button", { name: "Разрешение: 1K" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Разрешение: 4K" })).toBeNull();
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "Новый черновик" } });
    await selectModel("NeiroHub Chat");
    expect(screen.getByRole("textbox", { name: "Задайте вопрос NeiroHub" })).toHaveValue("Новый черновик");
    expect(screen.queryByRole("button", { name: /^Разрешение:/ })).toBeNull();
    await selectModel("Nano Banana Pro");
    expect(screen.getByRole("textbox")).toHaveValue("Новый черновик");
    expect(screen.getByRole("link", { name: "Все нейросети" })).toHaveAttribute("href", "/app/models");
    expect(push).not.toHaveBeenCalled();
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });

  it("prepares the selected model and settings on the home page and locks switching until confirmation completes", async () => {
    let finish!: (response: Response) => void;
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise(resolve => { finish = resolve; }));
    renderHero();
    await selectModel("Nano Banana Pro");
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "  Бумажный журавль  " } });
    fireEvent.click(screen.getByRole("button", { name: "Разрешение: 2K" }));
    fireEvent.click(screen.getByRole("radio", { name: "4K" }));
    fireEvent.click(screen.getByRole("button", { name: "Соотношение сторон: 16:9" }));
    fireEvent.click(screen.getByRole("radio", { name: "1:1" }));
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.generate }));
    expect(screen.getByRole("button", { name: "Выбрать модель: Nano Banana 2" })).toBeDisabled();
    expect(webBrowserMutation).toHaveBeenCalledExactlyOnceWith("/web/v1/image-jobs/prepare", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Idempotency-Key": requestId },
      body: JSON.stringify({ prompt: "Бумажный журавль", model_id: "nano-banana-pro", image_quality: "4K", aspect_ratio: "1:1", output_count: 1 }),
    });
    finish(Response.json({ job: {
      id: requestId, status: "prepared", prompt: "Бумажный журавль", model_id: "nano-banana-pro", model_name: "Nano Banana Pro", image_quality: "4K", cost_estimate: 60,
      created_at: "2026-09-12T00:00:00Z", updated_at: "2026-09-12T00:00:00Z",
    }, balance: 100, can_afford: true }, { status: 201 }));
    expect(await screen.findByRole("heading", { name: ru.imageGeneration.confirmationTitle })).toBeVisible();
    expect(screen.getByRole("button", { name: "Выбрать модель: NeiroHub Chat" })).toBeDisabled();
    expect(push).not.toHaveBeenCalled();
  });

  it("retains the draft after an error and retries the same generation with the same key", async () => {
    vi.mocked(webBrowserMutation).mockRejectedValue(new Error("offline"));
    renderHero();
    await selectModel("Nano Banana 2");
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "Сохрани текст" } });
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.generate }));
    await screen.findByRole("alert");
    expect(screen.getByRole("textbox")).toHaveValue("Сохрани текст");
    await waitFor(() => expect(screen.getByRole("button", { name: "Выбрать модель: Nano Banana Pro" })).toBeEnabled());
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.generate }));
    await waitFor(() => expect(webBrowserMutation).toHaveBeenCalledTimes(2));
    expect(vi.mocked(webBrowserMutation).mock.calls[1]).toEqual(vi.mocked(webBrowserMutation).mock.calls[0]);
  });

  it("lets a guest switch and type but requires login before preparing a generation", async () => {
    renderHero("guest");
    await selectModel("Nano Banana Pro");
    expect(push).not.toHaveBeenCalled();
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "Пример" } });
    fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.generate }));
    expect(push).toHaveBeenCalledExactlyOnceWith("/ru/login");
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });

  it("still opens a conversation when the text mode is submitted", async () => {
    renderHero();
    await selectModel("Nano Banana 2");
    await selectModel("NeiroHub Chat");
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "Помоги с текстом" } });
    fireEvent.click(screen.getByRole("button", { name: ru.workspace.promptSubmit }));
    expect(push).toHaveBeenCalledExactlyOnceWith(`/ru/app/chat/${requestId}?pending=1`);
    expect(readPendingConversationBootstrap(requestId)).toMatchObject({
      modelId: chatModel.id,
      prompt: "Помоги с текстом",
    });
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });
});
