import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { ProfileReferralProgram } from "./ProfileReferralProgram";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));
afterEach(() => { vi.clearAllMocks(); vi.unstubAllGlobals(); });
const summary = { code: "INVITE1234", visits: 7, registered: 3, activated: 0, rewarded: 0, rewards_enabled: false };

describe("ProfileReferralProgram", () => {
 it("reuses legacy invitation codes containing supported separators", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ ...summary, code: "WEB_AB-CD" }));
  render(<ProfileReferralProgram />);
  expect(await screen.findByRole("textbox", { name: "Ссылка-приглашение" })).toHaveValue(`${window.location.origin}/ru/invite/WEB_AB-CD`);
 });
 it("loads actual account counters and copies the same-origin invitation", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(summary));
  const writeText = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
  render(<ProfileReferralProgram />);
  const field = await screen.findByRole("textbox", { name: "Ссылка-приглашение" });
  expect(field).toHaveValue(`${window.location.origin}/ru/invite/INVITE1234`);
  expect(screen.getByText("7")).toBeInTheDocument();
  expect(within(screen.getByRole("heading", { name: "Регистрации" }).closest("article")!).getByText("3")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Скопировать приглашение" }));
  await waitFor(() => expect(writeText).toHaveBeenCalledWith(`${window.location.origin}/ru/invite/INVITE1234`));
  expect(await screen.findByRole("status")).toHaveTextContent("Ссылка скопирована");
  expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/referrals", expect.objectContaining({ cache: "no-store" }));
 });
 it("shows a retry after an outage without inventing counters", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null, { status: 503 })).mockResolvedValueOnce(Response.json(summary));
  render(<ProfileReferralProgram />);
  expect(await screen.findByRole("alert")).toHaveTextContent("Не удалось загрузить приглашение");
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
  expect(await screen.findByRole("textbox")).toBeInTheDocument();
 });
 it("rejects malformed codes or counters from the API", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ ...summary, code: "../../outside", visits: -1 }));
  render(<ProfileReferralProgram />);
  expect(await screen.findByRole("alert")).toBeInTheDocument();
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
 });
 it("keeps preview free of real requests, links and invented counts", () => {
  render(<ProfileReferralProgram preview />);
  expect(webBrowserFetch).not.toHaveBeenCalled();
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  expect(screen.getByText("Статистика доступна после входа в настоящий аккаунт.")).toBeInTheDocument();
 });
 it("removes previous counters when reloading fails", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json(summary)).mockResolvedValueOnce(new Response(null, { status: 503 }));
  const { rerender } = render(<ProfileReferralProgram />);
  expect(await screen.findByRole("textbox")).toBeInTheDocument();
  rerender(<ProfileReferralProgram preview />);
  rerender(<ProfileReferralProgram />);
  expect(await screen.findByRole("alert")).toBeInTheDocument();
  expect(screen.queryByText("7")).not.toBeInTheDocument();
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
 });
});
