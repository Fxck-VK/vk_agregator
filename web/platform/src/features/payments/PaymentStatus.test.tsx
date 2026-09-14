import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { PaymentStatus } from "./PaymentStatus";
import { getPayment, savePendingPayment } from "./payments";

const refresh = vi.hoisted(() => vi.fn());
vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh }) }));
vi.mock("./payments", async (original) => ({ ...await original<typeof import("./payments")>(), getPayment: vi.fn() }));
const id = "12345678-1234-4000-8000-123456789012";
const base = { id, amount: 40000, currency: "RUB" as const, credits: 800 };
describe("confirmed payment status", () => {
 beforeEach(() => { sessionStorage.clear(); vi.clearAllMocks(); });
 afterEach(() => { cleanup(); vi.useRealTimers(); });
 it("refreshes balance only after server-confirmed success", async () => {
  vi.useFakeTimers(); savePendingPayment(id);
  vi.mocked(getPayment).mockResolvedValueOnce({ ...base, status: "waiting_for_user" }).mockResolvedValue({ ...base, status: "succeeded" });
  render(<PaymentStatus paymentID={id} />);
  await act(async () => { await Promise.resolve(); });
  expect(refresh).not.toHaveBeenCalled();
  expect(sessionStorage.getItem("neirohub:pending-payment")).toBe(id);
  await act(async () => { await vi.advanceTimersByTimeAsync(3000); });
  expect(screen.getByText("Начислено 800 токенов.")).toBeInTheDocument();
  expect(refresh).toHaveBeenCalledTimes(1);
  expect(sessionStorage.getItem("neirohub:pending-payment")).toBeNull();
 });
 it("does not turn cancellation into a balance update", async () => {
  vi.mocked(getPayment).mockResolvedValue({ ...base, status: "canceled" });
  render(<PaymentStatus paymentID={id} />);
  expect(await screen.findByText("Оплата отменена. Токены не начислены.")).toBeInTheDocument();
  expect(refresh).not.toHaveBeenCalled();
 });
 it("allows another status check after a network failure without creating a payment", async () => {
  vi.mocked(getPayment).mockRejectedValueOnce(new Error("offline")).mockResolvedValue({ ...base, status: "succeeded" });
  render(<PaymentStatus paymentID={id} />);
  fireEvent.click(await screen.findByRole("button", { name: "Проверить оплату" }));
  await waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
 });
});
