import { webcrypto } from "node:crypto";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { PresentationChat } from "./PresentationChat";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn(), webBrowserMutation: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn(), push: vi.fn() }) }));
vi.mock("@/features/release/ReleaseViewProvider", () => ({ useReleaseView: () => ({ allowsPath: () => true }) }));
vi.mock("@/features/account/WorkspaceAccount/WorkspaceAccount", () => ({
  useWorkspaceAccountSnapshot: () => ({ profile: { account_id: "20000000-0000-4000-8000-000000000007" } }),
}));

const chat = "20000000-0000-4000-8000-000000000001";
const firstTurn = "30000000-0000-4000-8000-000000000001";
const secondTurn = "30000000-0000-4000-8000-000000000002";
const deckConversation = "20000000-0000-4000-8000-000000000003";
const deckJob = "30000000-0000-4000-8000-000000000003";
const endpoint = `/web/v1/presentation-chats/${chat}`;
const quote = { model_id: "qwen_3_7_flash", model_name: "Qwen 3.7 Flash", credits: 5, turn_credits: 3 };
const acceptedConsent = { model_id: quote.model_id, presentation_credits: 5, turn_credits: 3 };
const generated = { conversation_id: deckConversation, job_id: deckJob, status: "succeeded" };

beforeEach(() => vi.stubGlobal("crypto", webcrypto));
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
  vi.unstubAllGlobals();
  sessionStorage.clear();
});

async function send(prompt: string) {
  const textbox = screen.getByRole("textbox");
  fireEvent.change(textbox, { target: { value: prompt } });
  const button = screen.getByRole("button", { name: "Отправить" });
  await waitFor(() => expect(button).toBeEnabled());
  fireEvent.click(button);
}

describe("presentation chat", () => {
  it("lists clarification in the chat and starts exactly one deck only after the server reports a complete brief", async () => {
    let deckCreated = false;
    vi.mocked(webBrowserFetch).mockImplementation(async path => {
      if (path === "/web/v1/presentation-chats/quote") return Response.json(quote);
      if (path === endpoint) return Response.json({});
      if (path === `${endpoint}/turns/${firstTurn}`) {
        return Response.json({ ...acceptedConsent, job_id: firstTurn, status: "succeeded", reply: "Для кого презентация?", missing: ["audience"] });
      }
      if (path === `${endpoint}/turns/${secondTurn}`) {
        return Response.json({
          ...acceptedConsent, job_id: secondTurn, status: "succeeded", reply: "Бриф готов.", missing: [],
          ...(deckCreated ? { presentation: generated } : {}),
        });
      }
      if (path.startsWith(`/web/v1/conversations/${chat}/messages`)) return Response.json({ kind: "presentation", items: [] });
      throw new Error("Unexpected request");
    });
    vi.mocked(webBrowserMutation).mockImplementation(async path => {
      if (path === `${endpoint}/messages`) {
        const messageCount = vi.mocked(webBrowserMutation).mock.calls.filter(([route]) => route === path).length;
        return Response.json({ job_id: messageCount === 1 ? firstTurn : secondTurn, status: "queued" }, { status: 201 });
      }
      if (path === `${endpoint}/turns/${secondTurn}/generate`) {
        deckCreated = true;
        return Response.json(generated, { status: 201 });
      }
      throw new Error("Unexpected mutation");
    });

    render(<PresentationChat conversationId={chat} />);
    await screen.findByLabelText("Ответ в чате: 3 звезды");
    await screen.findByLabelText("Презентация после брифа: 5 звёзд");
    await send("Подготовь презентацию про школьный музей.");
    expect(await screen.findByText("Для кого презентация?")).toBeVisible();
    expect(vi.mocked(webBrowserMutation).mock.calls.map(([path]) => path)).toEqual([`${endpoint}/messages`]);

    await send("Для учеников седьмых классов.");
    const result = await screen.findByRole("link", { name: "Открыть презентацию" });
    expect(result).toHaveAttribute("href", `/ru/app/presentations?run=${deckConversation}&job=${deckJob}`);
    const [turnPath, turnArgs] = vi.mocked(webBrowserMutation).mock.calls.at(-1)!;
    expect(turnPath).toBe(`${endpoint}/turns/${secondTurn}/generate`);
    expect(JSON.parse(String(turnArgs.body))).toEqual({ model_id: "qwen_3_7_flash", expected_presentation_credits: 5 });
    expect(vi.mocked(webBrowserMutation)).toHaveBeenCalledTimes(3);
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  });

  it("reopens a ready brief using the earlier turn’s frozen consent", async () => {
    vi.mocked(webBrowserFetch).mockImplementation(async path => {
      if (path === "/web/v1/presentation-chats/quote") return Response.json({ ...quote, credits: 40, model_id: "updated_model" });
      if (path === endpoint) return Response.json({ ...acceptedConsent, job_id: secondTurn, status: "succeeded", missing: [] });
      if (path.startsWith(`/web/v1/conversations/${chat}/messages`)) return Response.json({ kind: "presentation", items: [] });
      throw new Error("Unexpected request");
    });
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json(generated, { status: 200 }));
    render(<PresentationChat conversationId={chat} />);
    await screen.findByRole("link", { name: "Открыть презентацию" });
    expect(vi.mocked(webBrowserMutation)).toHaveBeenCalledTimes(1);
    const [path, args] = vi.mocked(webBrowserMutation).mock.calls[0];
    expect(path).toBe(`${endpoint}/turns/${secondTurn}/generate`);
    expect(JSON.parse(String(args.body))).toEqual({ model_id: "qwen_3_7_flash", expected_presentation_credits: 5 });
  });

  it("requires approval for an updated deck quote without submitting another paid chat turn", async () => {
    const revisedQuote = { ...quote, model_id: "new_model", credits: 40 };
    vi.mocked(webBrowserFetch).mockImplementation(async path => {
      if (path === "/web/v1/presentation-chats/quote") return Response.json(revisedQuote);
      if (path === endpoint) return Response.json({ ...acceptedConsent, job_id: secondTurn, status: "succeeded", missing: [] });
      if (path.startsWith(`/web/v1/conversations/${chat}/messages`)) return Response.json({ kind: "presentation", items: [] });
      throw new Error("Unexpected request");
    });
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(Response.json({ error: "presentation quote changed" }, { status: 409 }))
      .mockResolvedValueOnce(Response.json(generated, { status: 201 }));
    render(<PresentationChat conversationId={chat} />);
    await screen.findByLabelText("Новая стоимость: 40 звёзд");
    expect(vi.mocked(webBrowserMutation)).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Запустить по новой стоимости" }));
    await screen.findByRole("link", { name: "Открыть презентацию" });
    expect(vi.mocked(webBrowserMutation)).toHaveBeenCalledTimes(2);
    const [path, args] = vi.mocked(webBrowserMutation).mock.calls[1];
    expect(path).toBe(`${endpoint}/turns/${secondTurn}/generate`);
    expect(JSON.parse(String(args.body))).toEqual({ model_id: "new_model", expected_presentation_credits: 40, accept_updated_quote: true });
  });

  it("resumes the same parked intake job only after an explicit retry", async () => {
    vi.mocked(webBrowserFetch).mockImplementation(async path => {
      if (path === "/web/v1/presentation-chats/quote") return Response.json(quote);
      if (path === endpoint) return Response.json({ ...acceptedConsent, job_id: firstTurn, status: "awaiting_payment" });
      if (path === `${endpoint}/turns/${firstTurn}`) {
        return Response.json({ ...acceptedConsent, job_id: firstTurn, status: "succeeded", reply: "Нужен стиль.", missing: ["style"] });
      }
      if (path.startsWith(`/web/v1/conversations/${chat}/messages`)) return Response.json({ kind: "presentation", items: [] });
      throw new Error("Unexpected request");
    });
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json({ job_id: firstTurn, status: "queued" }, { status: 201 }));
    render(<PresentationChat conversationId={chat} />);
    const retry = await screen.findByRole("button", { name: "Повторить отправку" });
    expect(vi.mocked(webBrowserMutation)).not.toHaveBeenCalled();
    fireEvent.click(retry);
    expect(await screen.findByText("Нужен стиль.")).toBeVisible();
    expect(vi.mocked(webBrowserMutation)).toHaveBeenCalledTimes(1);
    expect(vi.mocked(webBrowserMutation)).toHaveBeenCalledWith(`${endpoint}/turns/${firstTurn}/resume`, expect.objectContaining({ method: "POST" }));
  });
});
