import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { WorkspaceAccountProvider } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import { createSubscriptionCheckout, loadSubscription } from "./subscriptions";
import { SubscriptionCheckout } from "./SubscriptionCheckout";
import { paymentAttemptKey } from "@/features/payments/payments";
const navigation = vi.hoisted(() => ({ open: vi.fn() }));
vi.mock("./subscriptions", async importOriginal => ({ ...await importOriginal<typeof import("./subscriptions")>(), loadSubscription: vi.fn(), createSubscriptionCheckout: vi.fn() }));
vi.mock("@/features/payments/checkout-navigation", () => ({ checkoutNavigation: navigation }));
vi.mock("@/features/payments/PaymentStatus", () => ({ PaymentStatus: () => <div>Статус оплаты с сервера</div> }));
const state = { subscription: null, balances: [], checkout_available: true, recurring_available: true, trial_used: false };
const profile = { account_id: "10000000-0000-4000-8000-000000000001", identity_refs: [] };
const payment = { id: "20000000-0000-4000-8000-000000000001", status: "waiting_for_user" as const, amount: 19900, currency: "RUB" as const, credits: 400, confirmation_url: "https://yoomoney.ru/checkout" };
afterEach(() => { cleanup(); vi.resetAllMocks(); sessionStorage.clear(); });
function show(preview = false) { return render(<WorkspaceAccountProvider snapshot={{ profile, balance: 100 }}><SubscriptionCheckout planID="lite" preview={preview}/></WorkspaceAccountProvider>); }
describe("subscription checkout", () => {
  it("blocks a previously opened Lite checkout after the server reports trial consumption", async () => {
    vi.mocked(loadSubscription).mockResolvedValue({...state,trial_used:true});
    show();
    expect(await screen.findByText("Пробный тариф Lite уже использован. Выберите другой тариф.")).toBeInTheDocument();
    const button=screen.getByRole("button",{name:"Перейти к оплате"});
    expect(button).toBeDisabled();fireEvent.click(button);
    expect(createSubscriptionCheckout).not.toHaveBeenCalled();
  });
  it("keeps preview read-only", () => {
    show(true);
    expect(loadSubscription).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Перейти к оплате" })).toBeDisabled();
    expect(createSubscriptionCheckout).not.toHaveBeenCalled();
  });
  it("requires server availability and starts with recurring consent unchecked", async () => {
    vi.mocked(loadSubscription).mockResolvedValue({ ...state, checkout_available: false, balances: [{ kind: "legacy", available: -25, reserved: 0 }] });
    show();
    expect(await screen.findByText("Оплата подписок сейчас недоступна.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Перейти к оплате" })).toBeDisabled();
    const consent = screen.getByRole("checkbox", { name: /Разрешаю автоматическое продление/ });
    expect(consent).not.toBeChecked();
    expect(consent).toHaveAccessibleName(/не менее 24 часов/);
    expect(consent).toHaveAccessibleName(/14 дней с первой неудачной попытки/);
    expect(consent).toHaveAccessibleName(/не более 14 попыток всего, включая первую/);
    expect(consent).toHaveAccessibleName(/После отключения автопродления новые попытки прекращаются/);
  });
  it("shows trial conversion and sends consent only after explicit choice", async () => {
    vi.mocked(loadSubscription).mockResolvedValue(state);
    vi.mocked(createSubscriptionCheckout).mockResolvedValue(payment);
    show();
    await waitFor(() => expect(screen.getByRole("button", { name: "Перейти к оплате" })).toBeEnabled());
    expect(screen.getByRole("checkbox")).not.toBeChecked();
    expect(screen.getByText(/549.*Start\+/)).toBeInTheDocument();
    expect(screen.queryByText(/Покупка пакета не включает автопродление/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(screen.getByRole("button", { name: "Перейти к оплате" }));
    await waitFor(() => expect(createSubscriptionCheckout).toHaveBeenCalledWith({ plan_code: "lite", auto_renew: true, retry_policy_version: 1 }, expect.any(String)));
    expect(navigation.open).toHaveBeenCalledWith(payment.confirmation_url);
    expect(screen.queryByText(/Подписка активирована/)).not.toBeInTheDocument();
  });
  it("omits retry consent when automatic renewal is unchecked", async () => {
    vi.mocked(loadSubscription).mockResolvedValue(state);
    vi.mocked(createSubscriptionCheckout).mockResolvedValue(payment);
    show();
    const button = screen.getByRole("button", { name: "Перейти к оплате" });
    await waitFor(() => expect(button).toBeEnabled());
    fireEvent.click(button);
    await waitFor(() => expect(createSubscriptionCheckout).toHaveBeenCalledWith({ plan_code: "lite", auto_renew: false }, expect.any(String)));
  });
  it("does not reuse a cached key from a different renewal policy", async () => {
    vi.mocked(loadSubscription).mockResolvedValue(state);
    vi.mocked(createSubscriptionCheckout).mockResolvedValue(payment);
    show();
    await waitFor(() => expect(screen.getByRole("button", { name: "Перейти к оплате" })).toBeEnabled());
    fireEvent.click(screen.getByRole("checkbox"));
    const legacyKey = paymentAttemptKey("subscription:lite:true");
    fireEvent.click(screen.getByRole("button", { name: "Перейти к оплате" }));
    await waitFor(() => expect(createSubscriptionCheckout).toHaveBeenCalledTimes(1));
    expect(vi.mocked(createSubscriptionCheckout).mock.calls[0][1]).not.toBe(legacyKey);
  });
  it("retains the same operation key on an ambiguous failed request", async () => {
    vi.mocked(loadSubscription).mockResolvedValue(state);
    vi.mocked(createSubscriptionCheckout).mockRejectedValue(new Error("offline"));
    show();
    const button = screen.getByRole("button", { name: "Перейти к оплате" });
    await waitFor(() => expect(button).toBeEnabled());
    fireEvent.click(button);
    await screen.findByRole("alert");
    fireEvent.click(button);
    await waitFor(() => expect(createSubscriptionCheckout).toHaveBeenCalledTimes(2));
    expect(vi.mocked(createSubscriptionCheckout).mock.calls[0]).toEqual(vi.mocked(createSubscriptionCheckout).mock.calls[1]);
  });
  it("retains opted-in policy and operation key on an ambiguous failed request", async () => {
    vi.mocked(loadSubscription).mockResolvedValue(state);
    vi.mocked(createSubscriptionCheckout).mockRejectedValue(new Error("offline"));
    show();
    const button = screen.getByRole("button", { name: "Перейти к оплате" });
    await waitFor(() => expect(button).toBeEnabled());
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(button);
    await screen.findByRole("alert");
    fireEvent.click(button);
    await waitFor(() => expect(createSubscriptionCheckout).toHaveBeenCalledTimes(2));
    expect(vi.mocked(createSubscriptionCheckout).mock.calls[0][0]).toEqual({ plan_code: "lite", auto_renew: true, retry_policy_version: 1 });
    expect(vi.mocked(createSubscriptionCheckout).mock.calls[0]).toEqual(vi.mocked(createSubscriptionCheckout).mock.calls[1]);
  });
  it.each(["canceled", "failed", "expired"] as const)("does not redirect or activate a %s payment", async status => {
    vi.mocked(loadSubscription).mockResolvedValue(state);
    vi.mocked(createSubscriptionCheckout).mockResolvedValue({ ...payment, status });
    show();
    const button = screen.getByRole("button", { name: "Перейти к оплате" });
    await waitFor(() => expect(button).toBeEnabled());
    fireEvent.click(button);
    expect(await screen.findByText("Статус оплаты с сервера")).toBeInTheDocument();
    expect(navigation.open).not.toHaveBeenCalled();
  });
});
