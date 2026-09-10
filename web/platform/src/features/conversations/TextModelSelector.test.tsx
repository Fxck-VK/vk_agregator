import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { ConversationComposer } from "./ConversationComposer/ConversationComposer";

vi.mock("@/lib/web-api/browser", () => ({webBrowserFetch: vi.fn()}));
afterEach(() => {cleanup();vi.clearAllMocks();});

it("submits the selected public model and shows its backend price", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({items:[
    {id:"chatgpt",name:"НейроХаб",estimate_credits:0},
    {id:"gpt_5_5",name:"GPT-5.5",estimate_credits:20,max_prompt_bytes:7680,max_output_tokens:2048},
  ]}));
  const send=vi.fn();
  render(<ConversationComposer contentVersion="1" forceScrollRequest={0} scrollContainer={null} onSubmit={send} />);
  await screen.findByRole("option",{name:"GPT-5.5 · 20 кредитов за ответ"});
  fireEvent.change(screen.getByLabelText("Текстовая модель"),{target:{value:"gpt_5_5"}});
  fireEvent.change(screen.getByRole("textbox"),{target:{value:"Synthetic prompt"}});
  fireEvent.submit(screen.getByRole("textbox").closest("form")!);
  expect(send).toHaveBeenCalledWith("Synthetic prompt","gpt_5_5");
  expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/text-models");
});

it("does not offer paid models when the catalog fails", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValue(new Response("",{status:503}));
  render(<ConversationComposer contentVersion="1" forceScrollRequest={0} scrollContainer={null} onSubmit={vi.fn()} />);
  await screen.findByText("Каталог дополнительных моделей временно недоступен.");
  expect(screen.getAllByRole("option")).toHaveLength(1);
});

it("loads the expanded catalog and submits each new public model", async () => {
  const added = [
    ["claude_opus_4_8", "Claude Opus 4.8", 25],
    ["gpt_5_6_terra", "GPT 5.6 Terra", 10],
    ["gpt_6_astra", "GPT 6 Astra", 35],
    ["claude_opus_5", "Claude Opus 5", 25],
    ["gemini_3_7_flash", "Gemini 3.7 Flash", 5],
    ["claude_fable_5_1", "Claude Fable 5.1", 90],
    ["claude_fable_5", "Claude Fable 5", 45],
    ["gemini_3_6_flash", "Gemini 3.6 Flash", 5],
  ] as const;
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({items:[
    {id:"chatgpt", name:"НейроХаб", estimate_credits:0},
    ...[["gpt_5_5","GPT-5.5",20], ["claude_opus_4_7","Claude Opus 4.7",20], ["gemini_3_1_pro","Gemini 3.1 Pro",10], ...added].map(([id,name,price]) => ({id,name,estimate_credits:price,max_prompt_bytes:7680,max_output_tokens:2048})),
  ]}));
  const send = vi.fn();
  render(<ConversationComposer contentVersion="1" forceScrollRequest={0} scrollContainer={null} onSubmit={send} />);
  await screen.findByRole("option", {name:"Claude Fable 5.1 · 90 кредитов за ответ"});
  expect(screen.getAllByRole("option")).toHaveLength(12);
  for (const [id] of added) {
    fireEvent.change(screen.getByLabelText("Текстовая модель"), {target:{value:id}});
    fireEvent.change(screen.getByRole("textbox"), {target:{value:"Synthetic prompt"}});
    fireEvent.submit(screen.getByRole("textbox").closest("form")!);
    expect(send).toHaveBeenLastCalledWith("Synthetic prompt", id);
  }
});
