import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { ConversationComposer } from "./ConversationComposer/ConversationComposer";
import { ConversationModelSelector, useConversationModelSelection } from "./ConversationModelSelector/ConversationModelSelector";
import { loadGenerationModelCatalog } from "@/features/models/generation-model-catalog";

vi.mock("@/features/models/generation-model-catalog", () => ({ loadGenerationModelCatalog: vi.fn() }));
afterEach(() => { cleanup(); vi.clearAllMocks(); window.sessionStorage.clear(); });

const paidModels = [
  ["gpt_5_5", "GPT-5.5", 20],
  ["claude_opus_4_7", "Claude Opus 4.7", 20],
  ["gemini_3_1_pro", "Gemini 3.1 Pro", 10],
  ["claude_opus_4_8", "Claude Opus 4.8", 25],
  ["gpt_5_6_terra", "GPT 5.6 Terra", 10],
  ["gpt_6_astra", "GPT 6 Astra", 35],
  ["claude_opus_5", "Claude Opus 5", 25],
  ["gemini_3_7_flash", "Gemini 3.7 Flash", 5],
  ["claude_fable_5_1", "Claude Fable 5.1", 90],
  ["claude_fable_5", "Claude Fable 5", 45],
  ["gemini_3_6_flash", "Gemini 3.6 Flash", 5],
] as const;
const catalog = {
  categoryErrors: {},
  default_model_id: "chatgpt",
  items: [
    { categories: ["popular", "text", "free", "study-work"], category: "text" as const, id: "chatgpt", name: "NeiroHub Chat", estimate_credits: 0 },
    ...paidModels.map(([id, name, estimate_credits]) => ({
      categories: ["popular", "text", "study-work"],
      category: "text" as const,
      id,
      name,
      estimate_credits,
      max_prompt_bytes: 7680,
      max_output_tokens: 2048,
    })),
  ],
};

function Composer({ send }: { send: (prompt: string, modelId: string) => void }) {
  const selection = useConversationModelSelection("test-dialogue");
  const selectedModel = selection.catalog?.items.find((model) => model.id === selection.selectedModelId);
  return <ConversationComposer contentVersion="1" forceScrollRequest={0} scrollContainer={null}
    modelSelector={<ConversationModelSelector disabled={false} selection={selection} />}
    generationModel={selectedModel}
    selectedModel={selectedModel}
    onSubmit={(prompt) => send(prompt, selection.selectedModelId)} />;
}

async function choose(name: string) {
  fireEvent.click(await screen.findByRole("button", { name: /Выбрана нейросеть/ }));
  const dialog = screen.getByRole("dialog", { name: "Выбор модели для диалога" });
  fireEvent.click(within(within(dialog).getByRole("region", { name: "Текст" })).getByRole("button", { name }));
  fireEvent.animationEnd(dialog);
}

it("uses description tooltips only in the composer adapter", () => {
  const description = "Описание модели для текущего диалога";
  render(<ConversationModelSelector disabled={false} selection={{
    catalog: { items: [{ categories: ["popular", "text", "free", "study-work"], id: "chat", name: "Chat", description, category: "text", estimate_credits: 0 }], default_model_id: "chat", categoryErrors: {} },
    selectedModelId: "chat", status: "ready", selectModel: vi.fn(),
  }} />);
  fireEvent.click(screen.getByRole("button", { name: /Выбрана нейросеть/ }));
  const option = within(screen.getByRole("region", { name: "Текст" })).getByRole("button", { name: "Chat" });
  expect(option).toHaveAccessibleDescription(description);
  expect(within(option).getByText(description)).not.toBeVisible();
  fireEvent.pointerEnter(option);
  expect(screen.getByRole("tooltip")).toHaveTextContent(description);
});

it.each(paidModels)("selects the curated paid model %s and uses its server price", async (id, name, price) => {
  vi.mocked(loadGenerationModelCatalog).mockResolvedValue({
    ...catalog,
    items: [catalog.items[0], catalog.items.find((model) => model.id === id)!],
  });
  const send = vi.fn();
  render(<Composer send={send} />);
  await screen.findByRole("button", { name: /Выбрана нейросеть NeiroHub Chat/ });
  const input = screen.getByRole("textbox");
  await choose(name);
  expect(screen.getByTestId("credit-star-icon")).toBeVisible();
  expect(screen.getByTestId("credit-star-icon").closest("p")).toHaveTextContent(`${price} за ответ · до 2 048 токенов ответа`);
  expect(screen.getByRole("textbox")).toBe(input);
  expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  fireEvent.change(input, { target: { value: "Synthetic prompt" } });
  fireEvent.submit(input.closest("form")!);
  expect(send).toHaveBeenLastCalledWith("Synthetic prompt", id);
  expect(loadGenerationModelCatalog).toHaveBeenCalledOnce();
});

it("keeps unavailable paid models out of the selector when the catalog fails", async () => {
  vi.mocked(loadGenerationModelCatalog).mockRejectedValueOnce(new Error("offline"));
  render(<Composer send={vi.fn()} />);
  await vi.waitFor(() => expect(loadGenerationModelCatalog).toHaveBeenCalledOnce());
  expect(screen.queryByText("GPT-5.5")).not.toBeInTheDocument();
  expect(screen.queryByText(/токенов за ответ/)).not.toBeInTheDocument();
});

it("enforces the selected model UTF-8 limit without losing the draft", async () => {
  vi.mocked(loadGenerationModelCatalog).mockResolvedValue(catalog);
  const send = vi.fn();
  render(<Composer send={send} />);
  await screen.findByRole("button", { name: /Выбрана нейросеть NeiroHub Chat/ });
  await choose("GPT-5.5");
  const input = screen.getByRole("textbox");
  const draft = "я".repeat(3841);
  fireEvent.change(input, { target: { value: draft } });
  fireEvent.submit(input.closest("form")!);
  expect(send).not.toHaveBeenCalled();
  expect(screen.getByRole("alert")).toHaveTextContent("Сообщение слишком длинное");
  expect(input).toHaveValue(draft);
  await choose("NeiroHub Chat");
  fireEvent.submit(input.closest("form")!);
  expect(send).toHaveBeenCalledWith(draft, "chatgpt");
});
