import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

import { loadGenerationModelCatalog } from "@/features/models/generation-model-catalog";
import { ru } from "@/i18n/ru";

import { NewChatPrompt } from "./NewChatPrompt";
import { LocaleProvider } from "@/i18n/LocaleProvider";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("@/features/models/generation-model-catalog", () => ({ loadGenerationModelCatalog: vi.fn() }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });

it.each(["ru", "en"] as const)("shows a zero price with the shared currency icon (%s)", async locale => {
  vi.mocked(loadGenerationModelCatalog).mockResolvedValueOnce(generationCatalog([{ id: "free", name: "Free", estimate_credits: 0 }]));
  render(<LocaleProvider locale={locale}><NewChatPrompt modelId="free" /></LocaleProvider>);
  const amount = await screen.findByLabelText(locale === "ru" ? "0 звёзд" : "0 stars");
  expect(amount.querySelector('[data-testid="credit-star-icon"]')).not.toBeNull();
  expect(amount.closest("p")).toHaveTextContent(locale === "ru" ? "Free · 0 за ответ" : "Free · 0 per response");
});

function generationCatalog(items: Array<{ id: string; name: string; estimate_credits?: number }>) {
  return {
    default_model_id: items[0]?.id ?? "",
    categoryErrors: {},
    items: items.map((model) => ({
      ...model,
      description: `${model.name}: описание из каталога`,
      category: "text" as const,
      categories: ["popular", "text", "study-work", ...(model.estimate_credits === 0 ? ["free"] : [])],
      isFree: model.estimate_credits === 0,
    })),
  } as Awaited<ReturnType<typeof loadGenerationModelCatalog>>;
}

it("validates the requested model, retains the draft and blocks sending while the next model loads", async () => {
  vi.mocked(loadGenerationModelCatalog).mockResolvedValueOnce(generationCatalog([{ id: "first", name: "First", estimate_credits: 20 }]));
  const { rerender } = render(<NewChatPrompt modelId="first" />);
  const amount = await screen.findByLabelText("20 звёзд");
  expect(amount.querySelector('[data-testid="credit-star-icon"]')).not.toBeNull();
  expect(amount.closest("p")).toHaveTextContent("First · 20 за ответ");
  const input = screen.getByRole("textbox");
  fireEvent.change(input, { target: { value: "Черновик" } });
  expect(screen.getByRole("button", { name: ru.workspace.promptSubmit })).toBeEnabled();

  vi.mocked(loadGenerationModelCatalog).mockReturnValueOnce(new Promise(() => {}));
  rerender(<NewChatPrompt modelId="second" />);
  expect(screen.getByRole("button", { name: ru.workspace.promptSubmit })).toBeDisabled();
  expect(input).toHaveValue("Черновик");
  expect(screen.queryByLabelText("20 звёзд")).toBeNull();
});

it.each(["missing", "failure"])("does not silently substitute a different model on %s", async (scenario) => {
  if (scenario === "failure") vi.mocked(loadGenerationModelCatalog).mockRejectedValueOnce(new Error("offline"));
  else vi.mocked(loadGenerationModelCatalog).mockResolvedValueOnce(generationCatalog([{ id: "first", name: "First" }]));
  render(<NewChatPrompt modelId="unknown" />);
  await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(ru.modelsCatalog.loadFailure));
  expect(screen.getByRole("button", { name: ru.workspace.promptSubmit })).toBeDisabled();
});
