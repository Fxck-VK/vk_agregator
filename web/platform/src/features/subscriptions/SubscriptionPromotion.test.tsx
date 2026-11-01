import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { useEffect } from "react";
import { afterEach, expect, it, vi } from "vitest";
vi.mock("./promotions", async original => ({ ...await original<object>(), requestPromotionQuote: vi.fn() }));
import { requestPromotionQuote } from "./promotions";
import { SubscriptionPromotionProvider, useSubscriptionPromotion } from "./SubscriptionPromotion";
import { beginBrowserSession, endBrowserSession } from "@/lib/web-api/browser-session-state";
const quote = { code: "FIRST15", revision: 1, percent: 15, plans: [{ plan_code: "start" as const, base_amount: 54900, amount: 46665, currency: "RUB" as const, stars: 1250 }] };
function Consumer() { const promotion = useSubscriptionPromotion(); return <><span>{promotion.quote?.code ?? "none"}</span><button onClick={() => void promotion.apply("FIRST15")} disabled={!promotion.enabled}>Apply</button><button onClick={promotion.clear}>Clear</button></>; }
afterEach(() => { cleanup(); vi.resetAllMocks(); endBrowserSession(); });
it("shares a verified quote between consumers and removes it explicitly", async () => {
  vi.mocked(requestPromotionQuote).mockResolvedValue(quote);
  render(<SubscriptionPromotionProvider accountId="one"><Consumer /></SubscriptionPromotionProvider>);
  fireEvent.click(screen.getByText("Apply")); await screen.findByText("FIRST15");
  fireEvent.click(screen.getByText("Clear")); expect(screen.getByText("none")).toBeInTheDocument();
});
it("never calls the server in local preview", () => {
  render(<SubscriptionPromotionProvider accountId="one" preview><Consumer /></SubscriptionPromotionProvider>);
  expect(screen.getByText("Apply")).toBeDisabled(); fireEvent.click(screen.getByText("Apply")); expect(requestPromotionQuote).not.toHaveBeenCalled();
});
it("clears a quote on logout, including repeated same-account sessions", async () => {
  beginBrowserSession("one"); vi.mocked(requestPromotionQuote).mockResolvedValue(quote);
  render(<SubscriptionPromotionProvider accountId="one"><Consumer /></SubscriptionPromotionProvider>);
  for (let index = 0; index < 2; index++) {
    fireEvent.click(screen.getByText("Apply")); await screen.findByText("FIRST15");
    await act(async () => { endBrowserSession(); beginBrowserSession("one"); });
    expect(screen.getByText("none")).toBeInTheDocument();
  }
});
it("ignores a late apply after the provider account changes", async () => {
  let finish!: (value: typeof quote) => void;
  vi.mocked(requestPromotionQuote).mockReturnValue(new Promise(resolve => { finish = resolve; }));
  const view = render(<SubscriptionPromotionProvider accountId="one"><Consumer /></SubscriptionPromotionProvider>);
  fireEvent.click(screen.getByText("Apply"));
  view.rerender(<SubscriptionPromotionProvider accountId="two"><Consumer /></SubscriptionPromotionProvider>);
  await act(async () => { finish(quote); }); expect(screen.getByText("none")).toBeInTheDocument();
});
it("does not remount the workspace when the session layer confirms its identity", async () => {
  const mounted = vi.fn();
  function WorkspaceChild() { useEffect(() => { mounted(); }, []); return <Consumer />; }
  render(<SubscriptionPromotionProvider accountId="one"><WorkspaceChild /></SubscriptionPromotionProvider>);
  expect(mounted).toHaveBeenCalledTimes(1);
  await act(async () => { beginBrowserSession("one"); });
  expect(mounted).toHaveBeenCalledTimes(1);
});
