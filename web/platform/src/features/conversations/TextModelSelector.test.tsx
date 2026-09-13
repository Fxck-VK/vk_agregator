import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { ConversationComposer } from "./ConversationComposer/ConversationComposer";
import { ConversationModelSelector, useConversationModelSelection } from "./ConversationModelSelector/ConversationModelSelector";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));
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
  default_model_id: "chatgpt",
  items: [
    { id: "chatgpt", name: "NeiroHub Chat", estimate_credits: 0 },
    ...paidModels.map(([id, name, estimate_credits]) => ({ id, name, estimate_credits, max_prompt_bytes: 7680, max_output_tokens: 2048 })),
  ],
};

function Composer({ send }: { send: (prompt: string, modelId: string) => void }) {
  const selection = useConversationModelSelection("test-dialogue");
  return <ConversationComposer contentVersion="1" forceScrollRequest={0} scrollContainer={null}
    modelSelector={<ConversationModelSelector disabled={false} selection={selection} />}
    selectedModel={selection.catalog?.items.find((model) => model.id === selection.selectedModelId)}
    onSubmit={(prompt) => send(prompt, selection.selectedModelId)} />;
}

async function choose(name: string) {
  fireEvent.click(await screen.findByRole("button", { name: /Выбрана нейросеть/ }));
  const dialog = screen.getByRole("dialog", { name: "Выбор модели для диалога" });
  fireEvent.click(within(dialog).getByText(name, { exact: true }).closest("button")!);
  fireEvent.animationEnd(dialog);
}

it("uses the shared composer selector for every paid model and displays server prices", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(catalog));
  const send = vi.fn();
  render(<Composer send={send} />);
  await screen.findByRole("button", { name: /Выбрана нейросеть NeiroHub Chat/ });
  const input = screen.getByRole("textbox");
  for (const [id, name, price] of paidModels) {
    await choose(name);
    expect(screen.getByText(new RegExp(`^${price} токенов за ответ`))).toBeVisible();
    expect(screen.getByRole("textbox")).toBe(input);
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    fireEvent.change(input, { target: { value: "Synthetic prompt" } });
    fireEvent.submit(input.closest("form")!);
    expect(send).toHaveBeenLastCalledWith("Synthetic prompt", id);
  }
  expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/chat-models");
});

it("keeps unavailable paid models out of the selector when the catalog fails", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValue(new Response("", { status: 503 }));
  render(<Composer send={vi.fn()} />);
  await vi.waitFor(() => expect(webBrowserFetch).toHaveBeenCalledOnce());
  expect(screen.queryByText("GPT-5.5")).not.toBeInTheDocument();
  expect(screen.queryByText(/токенов за ответ/)).not.toBeInTheDocument();
});

it("enforces the selected model UTF-8 limit without losing the draft", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(catalog));
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
