import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cancelSubscription, loadSubscription, type SubscriptionState } from "./subscriptions";
import { ProfileSubscriptionCard } from "./ProfileSubscriptionCard";
vi.mock("./subscriptions", async importOriginal => ({ ...await importOriginal<typeof import("./subscriptions")>(), loadSubscription: vi.fn(), cancelSubscription: vi.fn() }));
const state: SubscriptionState = {
  subscription: { plan_code: "lite", plan_name: "Lite", amount: 19900, stars: 400, currency: "RUB", period_start: "2026-10-27T00:00:00Z", paid_until: "2026-11-03T00:00:00Z", renewal_at: "2026-11-03T00:00:00Z", next_plan_code: "start", next_amount: 54900, auto_renew: true, status: "active" },
  balances: [{ kind: "subscription", available: 300, reserved: 100, expires_at: "2026-11-03T00:00:00Z" }, { kind: "purchased", available: 50, reserved: 0 }], checkout_available: true, recurring_available: true, trial_used: true,
};
afterEach(() => { cleanup(); vi.resetAllMocks(); });
describe("profile subscription state", () => {
  it("never invents a subscription in preview", () => {
    render(<ProfileSubscriptionCard preview/>);
    expect(loadSubscription).not.toHaveBeenCalled();
    expect(screen.queryByText("Lite")).not.toBeInTheDocument();
  });
  it("shows server plan, next amount, lot expiry and purchased balance", async () => {
    vi.mocked(loadSubscription).mockResolvedValue(state);
    render(<ProfileSubscriptionCard/>);
    expect(await screen.findByRole("heading", { name: "Lite" })).toBeInTheDocument();
    expect(screen.getByText(/549/)).toBeInTheDocument();
    expect(screen.getByText("Купленные звёзды")).toBeInTheDocument();
    expect(screen.getAllByText(/3 нояб.*2026/).length).toBeGreaterThan(0);
  });
  it("shows an unpaid renewal's actual next attempt from renewal_at", async () => {
    vi.mocked(loadSubscription).mockResolvedValue({ ...state, subscription: { ...state.subscription!, status: "past_due", renewal_at: "2026-11-05T12:00:00Z" } });
    render(<ProfileSubscriptionCard/>);
    expect(await screen.findByText("Продление не оплачено")).toBeInTheDocument();
    const nextCharge = screen.getByText("Следующее списание").closest("div")!;
    expect(within(nextCharge).getByText(/549.*5 нояб.*2026/)).toBeInTheDocument();
    expect(within(nextCharge).queryByText(/3 нояб/)).not.toBeInTheDocument();
  });
  it("shows a signed legacy balance as an adjustment without hiding the subscription", async () => {
    vi.mocked(loadSubscription).mockResolvedValue({ ...state, balances: [{ kind: "legacy", available: -25, reserved: 0 }] });
    render(<ProfileSubscriptionCard/>);
    expect(await screen.findByRole("heading", { name: "Lite" })).toBeInTheDocument();
    expect(screen.getByText(/Корректировка баланса/)).toBeInTheDocument();
    expect(screen.getByLabelText("-25 звёзд")).toBeInTheDocument();
  });
  it("uses confirmation and displays the server cancellation without resetting balances", async () => {
    vi.mocked(loadSubscription).mockResolvedValue(state);
    vi.mocked(cancelSubscription).mockResolvedValue({ ...state, subscription: { ...state.subscription!, auto_renew: false, status: "cancelled" } });
    render(<ProfileSubscriptionCard/>);
    fireEvent.click(await screen.findByRole("button", { name: "Отключить автопродление" }));
    const dialog = screen.getByRole("dialog", { name: "Отключить автопродление?" });
    expect(cancelSubscription).not.toHaveBeenCalled();
    fireEvent.click(within(dialog).getByRole("button", { name: "Отключить автопродление" }));
    await waitFor(() => expect(cancelSubscription).toHaveBeenCalledTimes(1));
    expect(await screen.findByText("Продление отменено")).toBeInTheDocument();
    expect(screen.getByText("Купленные звёзды")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Отключить автопродление" })).not.toBeInTheDocument();
  });
});
