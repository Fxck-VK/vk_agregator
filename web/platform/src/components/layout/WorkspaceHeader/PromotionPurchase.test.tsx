import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
vi.mock("@/features/subscriptions/promotions", async original => ({ ...await original<object>(), requestPromotionQuote: vi.fn() }));
vi.mock("@/features/subscriptions/subscriptions", () => ({ loadSubscription: vi.fn(), createSubscriptionCheckout: vi.fn() }));
vi.mock("@/features/payments/checkout-navigation", () => ({ checkoutNavigation: { open: vi.fn() } }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
import { requestPromotionQuote } from "@/features/subscriptions/promotions";
import { loadSubscription, createSubscriptionCheckout } from "@/features/subscriptions/subscriptions";
import { checkoutNavigation } from "@/features/payments/checkout-navigation";
import { PaymentRequestError } from "@/features/payments/payments";
import { beginBrowserSession, endBrowserSession } from "@/lib/web-api/browser-session-state";
import { WorkspaceAccountProvider } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import { SubscriptionPlansDialog } from "./SubscriptionPlansDialog";

const quote = { code: "FIRST15", revision: 3, percent: 15, plans: [{ plan_code: "start" as const, base_amount: 54900, amount: 46665, currency: "RUB" as const, stars: 1250 }] };
const profile = { account_id: "10000000-0000-4000-8000-000000000001", identity_refs: [] };
const state = { subscription: null, balances: [], checkout_available: true, recurring_available: true, trial_used: false };
const payment = { id: "20000000-0000-4000-8000-000000000001", amount: 46665, currency: "RUB" as const, credits: 1250, status: "waiting_for_user" as const, confirmation_url: "https://yookassa.ru/checkout/synthetic" };
beforeEach(() => { vi.mocked(loadSubscription).mockResolvedValue(state); vi.mocked(requestPromotionQuote).mockResolvedValue(quote); vi.mocked(createSubscriptionCheckout).mockResolvedValue(payment); });
afterEach(() => { cleanup(); vi.resetAllMocks(); sessionStorage.clear(); endBrowserSession(); });
function show(preview = false) { render(<WorkspaceAccountProvider snapshot={{ profile, balance: 100 }} preview={preview}><SubscriptionPlansDialog onClose={vi.fn()} /></WorkspaceAccountProvider>); }
function startCard() { return screen.getByRole("heading", { name: "Start+", hidden: true }).closest("article")!; }
async function applyCode() {
  fireEvent.click(screen.getByRole("button", { name: "Активировать промокод" }));
  const dialog = screen.getByRole("dialog", { name: "У меня есть промокод" });
  fireEvent.change(within(dialog).getByRole("textbox", { name: "Промокод" }), { target: { value: "first15" } });
  fireEvent.click(within(dialog).getByRole("button", { name: "Активировать" }));
  await waitFor(() => expect(screen.queryByRole("dialog", { name: "У меня есть промокод" })).not.toBeInTheDocument());
}
it("applies the same verified discount to the card and checkout, preserving full stars and regular renewal", async () => {
  show(); await applyCode();
  expect(startCard()).toHaveTextContent("466,65"); expect(startCard()).toHaveTextContent("1 250");
  fireEvent.click(screen.getByRole("checkbox", { name: "Автопродление подписки" }));
  expect(screen.getByText(/Lite → Start\+:/)).toHaveTextContent("549");
  fireEvent.click(within(startCard()).getByRole("button", { name: "Активировать подписку" }));
  await waitFor(() => expect(createSubscriptionCheckout).toHaveBeenCalledWith({ plan_code: "start", auto_renew: true, retry_policy_version: 1, promo_code: "FIRST15", promo_revision: 3 }, expect.any(String)));
  expect(checkoutNavigation.open).toHaveBeenCalledWith(payment.confirmation_url);
});
it("blocks navigation if checkout returns the undiscounted amount after applying a code", async () => {
  vi.mocked(createSubscriptionCheckout).mockResolvedValue({ ...payment, amount: 54900 });
  show(); await applyCode(); fireEvent.click(within(startCard()).getByRole("button", { name: "Активировать подписку" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(/Условия изменились/);
  expect(checkoutNavigation.open).not.toHaveBeenCalled();
});
it("does not open or remember an old checkout after the same account starts a new session", async () => {
  beginBrowserSession(profile.account_id);
  let finish!: (value: typeof payment) => void;
  vi.mocked(createSubscriptionCheckout).mockReturnValue(new Promise(resolve => { finish = resolve; }));
  show(); await applyCode();
  fireEvent.click(within(startCard()).getByRole("button", { name: "Активировать подписку" }));
  await waitFor(() => expect(createSubscriptionCheckout).toHaveBeenCalledOnce());
  await act(async () => { endBrowserSession(); beginBrowserSession(profile.account_id); finish(payment); });
  expect(checkoutNavigation.open).not.toHaveBeenCalled();
  expect(sessionStorage.getItem("neirohub:pending-payment")).toBeNull();
});
it("removes an applied code and restores ordinary checkout terms", async () => {
  show(); await applyCode();
  fireEvent.click(screen.getByRole("button", { name: "Промокод FIRST15 · −15%" }));
  fireEvent.click(screen.getByRole("button", { name: "Убрать промокод" }));
  expect(startCard()).toHaveTextContent("549"); expect(startCard()).not.toHaveTextContent("466,65");
  vi.mocked(createSubscriptionCheckout).mockResolvedValue({ ...payment, amount: 54900 });
  fireEvent.click(within(startCard()).getByRole("button", { name: "Активировать подписку" }));
  await waitFor(() => expect(createSubscriptionCheckout).toHaveBeenCalledWith({ plan_code: "start", auto_renew: false }, expect.any(String)));
});
it("does not validate real codes in local preview", () => {
  show(true); fireEvent.click(screen.getByRole("button", { name: "Активировать промокод" }));
  const dialog = screen.getByRole("dialog", { name: "У меня есть промокод" });
  expect(dialog).toHaveTextContent("Локальный предпросмотр: промокоды не применяются");
  fireEvent.change(within(dialog).getByRole("textbox", { name: "Промокод" }), { target: { value: "FIRST15" } });
  expect(within(dialog).getByRole("button", { name: "Активировать" })).toBeDisabled();
  expect(requestPromotionQuote).not.toHaveBeenCalled();
});
it("keeps an unavailable or expired code out of prices and shows the server rejection", async () => {
  vi.mocked(requestPromotionQuote).mockRejectedValue(new PaymentRequestError(409));
  show(); fireEvent.click(screen.getByRole("button", { name: "Активировать промокод" }));
  const dialog = screen.getByRole("dialog", { name: "У меня есть промокод" });
  fireEvent.change(within(dialog).getByRole("textbox"), { target: { value: "FIRST15" } });
  fireEvent.click(within(dialog).getByRole("button", { name: "Активировать" }));
  expect(await within(dialog).findByRole("alert")).toHaveTextContent(/недоступен/);
  expect(startCard()).not.toHaveTextContent("466,65");
  expect(createSubscriptionCheckout).not.toHaveBeenCalled();
});
