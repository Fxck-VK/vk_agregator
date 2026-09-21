import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { TokenTopUpDialog } from "./TokenTopUpDialog";
import { createPayment, loadPaymentProducts, PaymentRequestError } from "@/features/payments/payments";
import { checkoutNavigation } from "@/features/payments/checkout-navigation";

vi.mock("@/features/payments/payments", async (original) => ({ ...await original<typeof import("@/features/payments/payments")>(), loadPaymentProducts: vi.fn(), createPayment: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh: vi.fn() }) }));

describe("token checkout", () => {
 beforeEach(() => {
  sessionStorage.clear(); vi.clearAllMocks();
  vi.mocked(loadPaymentProducts).mockResolvedValue({ items: [{ code: "custom", amount: 12300, currency: "RUB", credits: 777 }], checkout_available: true });
 });
 afterEach(() => { cleanup(); vi.restoreAllMocks(); });
 it("buys the server package and retries the same operation after an uncertain response", async () => {
  const open = vi.spyOn(checkoutNavigation, "open").mockImplementation(() => {});
  const payment = { id: "12345678-1234-4000-8000-123456789012", status: "waiting_for_user" as const, amount: 12300, currency: "RUB" as const, credits: 777, confirmation_url: "https://yoomoney.ru/checkout/test" };
  vi.mocked(createPayment).mockRejectedValueOnce(new Error("network")).mockResolvedValueOnce(payment);
  render(<TokenTopUpDialog onClose={vi.fn()} />);
  expect(await screen.findByRole("radio", { name: "777 звёзд за 123 ₽" })).toBeChecked();
  expect(screen.getByLabelText("777 звёзд")).toContainElement(screen.getByTestId("credit-star-icon"));
  expect(screen.queryByText("ТОКЕНОВ")).not.toBeInTheDocument();
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Купить за 123 ₽" }));
  await screen.findByRole("alert");
  fireEvent.click(screen.getByRole("button", { name: "Купить за 123 ₽" }));
  await waitFor(() => expect(open).toHaveBeenCalledWith(payment.confirmation_url));
  expect(vi.mocked(createPayment).mock.calls[0]).toEqual(vi.mocked(createPayment).mock.calls[1]);
  expect(vi.mocked(createPayment).mock.calls[0][0]).toEqual({ product_code: "custom" });
  expect(sessionStorage.getItem("neirohub:pending-payment")).toBe(payment.id);
 });
 it("does not offer fake packages when catalog is unavailable", async () => {
  vi.mocked(loadPaymentProducts).mockRejectedValue(new Error("offline"));
  render(<TokenTopUpDialog onClose={vi.fn()} />);
  expect(await screen.findByRole("alert")).toBeInTheDocument();
  expect(screen.queryAllByRole("radio")).toHaveLength(0);
  expect(createPayment).not.toHaveBeenCalled();
 });
 it("explains a missing verified account email without adding a receipt input", async () => {
  vi.mocked(createPayment).mockRejectedValue(new PaymentRequestError(422));
  render(<TokenTopUpDialog onClose={vi.fn()} />);
  fireEvent.click(await screen.findByRole("button", { name: "Купить за 123 ₽" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("подтверждённый email в аккаунте");
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
 });
 it("blocks purchases until a test shop is configured", async () => {
  vi.mocked(loadPaymentProducts).mockResolvedValue({ items: [{ code: "custom", amount: 12300, currency: "RUB", credits: 777 }], checkout_available: false });
  render(<TokenTopUpDialog onClose={vi.fn()} />);
  expect(await screen.findByRole("button", { name: "Купить за 123 ₽" })).toBeDisabled();
 });
});
