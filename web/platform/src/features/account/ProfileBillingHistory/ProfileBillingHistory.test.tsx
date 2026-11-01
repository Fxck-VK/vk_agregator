import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderToString } from "react-dom/server";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { beginBrowserSession, endBrowserSession } from "@/lib/web-api/browser-session-state";
import { ProfileBillingHistory } from "./ProfileBillingHistory";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));
const purchase = { id: "payment:10000000-0000-4000-8000-000000000001", kind: "topup", created_at: "2026-10-31T09:13:00Z", stars_delta: 777, status: "succeeded", money: { amount_minor: 12345, currency: "rub" } };
const debit = { id: "ledger:10000000-0000-4000-8000-000000000002", kind: "generation", created_at: "2026-10-30T09:13:00Z", stars_delta: -12, status: "succeeded", modality: "image", operation: "image_generate" };
const page = (items: unknown[] = [purchase], next_cursor: string | null = null) => Response.json({ items, next_cursor });
beforeEach(() => beginBrowserSession("account-a"));
afterEach(() => { act(() => endBrowserSession()); vi.resetAllMocks(); });

describe("ProfileBillingHistory", () => {
	 it("preserves the canonical Start+ tariff name", async () => {
	   vi.mocked(webBrowserFetch).mockResolvedValue(page([{ ...purchase, kind: "subscription", plan_code: "start" }]));
	   render(<ProfileBillingHistory accountId="account-a" />);
	   expect(await screen.findByText("Покупка подписки · Start+")).toBeInTheDocument();
	 });
  it("renders an honest loading state before hydration without reading account history", () => {
    const html = renderToString(<ProfileBillingHistory accountId="account-a" />);
    expect(html).toContain("Загружаем историю…");
    expect(html).not.toContain("Войдите в аккаунт");
    expect(webBrowserFetch).not.toHaveBeenCalled();
  });
  it("labels preview explicitly without requesting or inventing financial records", () => {
    render(<ProfileBillingHistory accountId="account-a" preview />);
    expect(screen.getByText(/предпросмотр.*история.*недоступна/i)).toBeInTheDocument();
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
    expect(webBrowserFetch).not.toHaveBeenCalled();
  });
  it("renders only safe localized historical money, signed stars, status and dates", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(page([{ ...purchase, test_mode: true, provider_id: "secret", reason: "secret", metadata: { prompt: "secret" } }, debit]));
    render(<ProfileBillingHistory accountId="account-a" />);
    expect(screen.getByText("Загружаем историю…")).toBeInTheDocument();
    expect(await screen.findByText("Пополнение звёзд")).toBeInTheDocument();
    expect(screen.getByText("123,45 ₽")).toBeInTheDocument();
    expect(screen.getByLabelText(/\+ 777/)).toBeInTheDocument();
    expect(screen.getByLabelText(/-12/)).toBeInTheDocument();
    expect(screen.getByText("Генерация изображения")).toBeInTheDocument();
    expect(screen.getByText("Тестовая операция")).toBeInTheDocument();
    expect(document.querySelectorAll("time[datetime]")).toHaveLength(2);
    expect(document.body).not.toHaveTextContent("secret");
  });
  it("retries an initial error and distinguishes a verified empty history", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null, { status: 503 })).mockResolvedValueOnce(page([]));
    render(<ProfileBillingHistory accountId="account-a" />);
    expect(await screen.findByRole("alert")).toHaveTextContent("Не удалось загрузить историю");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(await screen.findByText("Покупок и списаний пока нет.")).toBeInTheDocument();
  });
  it("preserves every stored kopek even at the maximum safe integer and hides a false test flag", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(page([{ ...purchase, stars_delta: 0, status: "pending", test_mode: false, money: { amount_minor: Number.MAX_SAFE_INTEGER, currency: "rub" } }]));
    render(<ProfileBillingHistory accountId="account-a" />);
    expect(await screen.findByText("90 071 992 547 409,91 ₽")).toBeInTheDocument();
    expect(screen.getByLabelText("0 звёзд")).toBeInTheDocument();
    expect(screen.getByText("В обработке")).toBeInTheDocument();
    expect(screen.queryByText("Тестовая операция")).not.toBeInTheDocument();
  });
  it("preserves rows after a page error, retries the same cursor and deduplicates records", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(page([purchase], "next_1")).mockResolvedValueOnce(new Response(null, { status: 503 })).mockResolvedValueOnce(page([purchase, debit]));
    render(<ProfileBillingHistory accountId="account-a" />);
    fireEvent.click(await screen.findByRole("button", { name: "Показать ещё" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Не удалось загрузить следующую страницу");
    expect(screen.getByText("Пополнение звёзд")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    await screen.findByText("Генерация изображения");
    expect(screen.getAllByRole("listitem")).toHaveLength(2);
    expect(vi.mocked(webBrowserFetch).mock.calls.slice(1).map(([url]) => url)).toEqual(["/web/v1/billing/history?limit=20&cursor=next_1", "/web/v1/billing/history?limit=20&cursor=next_1"]);
    expect(screen.queryByRole("button", { name: "Показать ещё" })).not.toBeInTheDocument();
  });
  it("rejects a repeated cursor without appending inconsistent rows", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(page([purchase], "same")).mockResolvedValueOnce(page([debit], "same"));
    render(<ProfileBillingHistory accountId="account-a" />);
    fireEvent.click(await screen.findByRole("button", { name: "Показать ещё" }));
    await screen.findByRole("alert");
    expect(screen.queryByText("Генерация изображения")).not.toBeInTheDocument();
  });
  it("clears already visible financial records immediately on logout without a parent rerender", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(page());
    render(<ProfileBillingHistory accountId="account-a" />);
    await screen.findByText("Пополнение звёзд");
    act(() => endBrowserSession());
    expect(screen.queryByText("Пополнение звёзд")).not.toBeInTheDocument();
  });
  it("discards late JSON on logout and fetches afresh when the same account signs in again", async () => {
    let finish!: (value: unknown) => void;
    vi.mocked(webBrowserFetch).mockResolvedValueOnce({ ok: true, json: () => new Promise(resolve => { finish = resolve; }) } as Response);
    render(<ProfileBillingHistory accountId="account-a" />);
    await waitFor(() => expect(finish).toBeTypeOf("function"));
    act(() => endBrowserSession());
    await act(async () => finish({ items: [purchase], next_cursor: null }));
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(page([debit]));
    await act(async () => beginBrowserSession("account-a"));
    expect(await screen.findByText("Генерация изображения")).toBeInTheDocument();
    expect(screen.queryByText("Пополнение звёзд")).not.toBeInTheDocument();
    expect(webBrowserFetch).toHaveBeenCalledTimes(2);
  });
  it.each(["initial", "page"])('discards late %s JSON after account/session change', async stage => {
    let finish!: (value: unknown) => void;
    const delayed = { ok: true, json: () => new Promise(resolve => { finish = resolve; }) } as Response;
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(stage === "page" ? page([purchase], "next") : delayed);
    if (stage === "page") vi.mocked(webBrowserFetch).mockResolvedValueOnce(delayed);
    const view = render(<ProfileBillingHistory accountId="account-a" />);
    if (stage === "page") fireEvent.click(await screen.findByRole("button", { name: "Показать ещё" }));
    await waitFor(() => expect(finish).toBeTypeOf("function"));
    const previousSignal = vi.mocked(webBrowserFetch).mock.calls.at(-1)![1]!.signal;
    act(() => beginBrowserSession("account-b"));
    vi.mocked(webBrowserFetch).mockReturnValue(new Promise(() => {}));
    view.rerender(<ProfileBillingHistory accountId="account-b" />);
    await act(async () => finish({ items: [purchase, debit], next_cursor: null }));
    expect(previousSignal!.aborted).toBe(true);
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
  });
});
