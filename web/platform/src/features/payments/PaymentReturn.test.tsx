import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { PaymentReturn } from "./PaymentReturn";
import { getPayment, savePendingPayment } from "./payments";

const refresh = vi.hoisted(() => vi.fn());
vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh }) }));
vi.mock("./payments", async (original) => ({ ...await original<typeof import("./payments")>(), getPayment: vi.fn() }));
const id = "12345678-1234-4000-8000-123456789012";
describe("payment return recovery", () => {
 afterEach(() => { cleanup(); sessionStorage.clear(); vi.clearAllMocks(); });
 it("checks a returned payment ID with the server instead of treating the URL as success", async () => {
  vi.mocked(getPayment).mockResolvedValue({ id, status: "canceled", amount: 40000, currency: "RUB", credits: 800 });
  render(<PaymentReturn paymentID={id} />);
  await screen.findByText("Оплата отменена. Токены не начислены.");
  expect(getPayment).toHaveBeenCalledWith(id, expect.any(AbortSignal));
  expect(refresh).not.toHaveBeenCalled();
 });
 it("recovers a pending payment across reload and keeps success visible after clearing storage", async () => {
  savePendingPayment(id);
  vi.mocked(getPayment).mockResolvedValue({ id, status: "succeeded", amount: 40000, currency: "RUB", credits: 800 });
  const { rerender } = render(<PaymentReturn />);
  await screen.findByLabelText("800 звёзд");
  expect(sessionStorage.getItem("neirohub:pending-payment")).toBeNull();
  rerender(<PaymentReturn />);
  expect(screen.getByTestId("credit-star-icon")).toBeInTheDocument();
 });
});
