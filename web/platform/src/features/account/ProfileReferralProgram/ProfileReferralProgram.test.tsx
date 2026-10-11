import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { renderToString } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { beginBrowserSession, endBrowserSession } from "@/lib/web-api/browser-session-state";
import { ProfileReferralProgram } from "./ProfileReferralProgram";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));
beforeEach(() => beginBrowserSession("account-a"));
afterEach(() => { act(() => endBrowserSession()); vi.resetAllMocks(); vi.unstubAllGlobals(); });
const summary = { code: "INVITE1234", visits: 7, registered: 3, activated: 0, rewarded: 0, rewards_enabled: false };

describe("ProfileReferralProgram", () => {
 it("shows the cached link immediately while reopening refreshes only statistics", async () => {
  let finish!: (response: Response) => void;
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json(summary)).mockReturnValueOnce(new Promise(resolve => { finish = resolve; }));
  const first = render(<ProfileReferralProgram accountId="account-a" />);
  expect(await screen.findByRole("textbox")).toHaveValue(`${window.location.origin}/ru/invite/INVITE1234`);
  first.unmount();
  render(<ProfileReferralProgram accountId="account-a" />);
  expect(screen.getByRole("textbox")).toHaveValue(`${window.location.origin}/ru/invite/INVITE1234`);
  expect(webBrowserFetch).toHaveBeenLastCalledWith("/web/v1/referrals/stats", expect.objectContaining({ cache: "no-store" }));
  expect(screen.queryByText("7")).not.toBeInTheDocument();
  await act(async () => finish(Response.json({ visits: 9, registered: 4, activated: 0, rewarded: 0, rewards_enabled: false })));
  expect(await screen.findByText("9")).toBeInTheDocument();
 });
 it("keeps the cached link usable when statistics fail and retries statistics alone", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json(summary)).mockResolvedValueOnce(new Response(null, { status: 503 })).mockResolvedValueOnce(Response.json({ ...summary, visits: 8 }));
  const first = render(<ProfileReferralProgram accountId="account-a" />);
  await screen.findByRole("textbox");
  first.unmount();
  render(<ProfileReferralProgram accountId="account-a" />);
  expect(await screen.findByRole("alert")).toHaveTextContent("Не удалось обновить статистику");
  expect(screen.getByRole("textbox")).toHaveValue(`${window.location.origin}/ru/invite/INVITE1234`);
  expect(screen.queryByText("7")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
  expect(await screen.findByText("8")).toBeInTheDocument();
  expect(webBrowserFetch).toHaveBeenLastCalledWith("/web/v1/referrals/stats", expect.anything());
 });
 it("clears the code on logout even if the same account signs in again", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json(summary)).mockReturnValueOnce(new Promise(() => {}));
  const first = render(<ProfileReferralProgram accountId="account-a" />);
  await screen.findByRole("textbox");
  first.unmount();
  endBrowserSession(); beginBrowserSession("account-a");
  render(<ProfileReferralProgram accountId="account-a" />);
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  expect(webBrowserFetch).toHaveBeenLastCalledWith("/web/v1/referrals", expect.anything());
 });
 it("does not display the previous account's link while another account loads", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json(summary)).mockReturnValueOnce(new Promise(() => {}));
  const view = render(<ProfileReferralProgram accountId="account-a" />);
  await screen.findByRole("textbox");
  act(() => { beginBrowserSession("account-b"); });
  view.rerender(<ProfileReferralProgram accountId="account-b" />);
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  expect(screen.queryByText("7")).not.toBeInTheDocument();
 });
 it("ignores a delayed response after the authenticated account changed", async () => {
  let finish!: (response: Response) => void;
  vi.mocked(webBrowserFetch).mockReturnValueOnce(new Promise(resolve => { finish = resolve; })).mockReturnValueOnce(new Promise(() => {}));
  const view = render(<ProfileReferralProgram accountId="account-a" />);
  act(() => { beginBrowserSession("account-b"); });
  view.rerender(<ProfileReferralProgram accountId="account-b" />);
  await act(async () => finish(Response.json(summary)));
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  expect(screen.queryByText("7")).not.toBeInTheDocument();
 });
 it("does not serialize a tab's cached code into server rendered markup", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json(summary));
  const view = render(<ProfileReferralProgram accountId="account-a" />);
  await screen.findByRole("textbox");
  view.unmount();
  const markup = renderToString(<ProfileReferralProgram accountId="account-a" />);
  expect(markup).not.toContain("INVITE1234");
  expect(markup).toContain("Загружаем приглашение");
 });
 it("does not use a cached real link or make reads in preview", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json(summary));
  const view = render(<ProfileReferralProgram accountId="account-a" />);
  await screen.findByRole("textbox");
  view.rerender(<ProfileReferralProgram accountId="account-a" preview />);
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  expect(webBrowserFetch).toHaveBeenCalledTimes(1);
 });
 it("reuses legacy invitation codes containing supported separators", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ ...summary, code: "WEB_AB-CD" }));
  render(<ProfileReferralProgram accountId="account-a" />);
  expect(await screen.findByRole("textbox", { name: "Ссылка-приглашение" })).toHaveValue(`${window.location.origin}/ru/invite/WEB_AB-CD`);
 });
 it("loads actual account counters and copies the same-origin invitation", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(summary));
  const writeText = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
  render(<ProfileReferralProgram accountId="account-a" />);
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
  render(<ProfileReferralProgram accountId="account-a" />);
  expect(await screen.findByRole("alert")).toHaveTextContent("Не удалось загрузить приглашение");
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
  expect(await screen.findByRole("textbox")).toBeInTheDocument();
 });
 it("rejects malformed codes or counters from the API", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ ...summary, code: "../../outside", visits: -1 }));
  render(<ProfileReferralProgram accountId="account-a" />);
  expect(await screen.findByRole("alert")).toBeInTheDocument();
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
 });
 it("keeps preview free of real requests, links and invented counts", () => {
  render(<ProfileReferralProgram accountId="account-a" preview />);
  expect(webBrowserFetch).not.toHaveBeenCalled();
  expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  expect(screen.getByText("Статистика доступна после входа в настоящий аккаунт.")).toBeInTheDocument();
 });
 it("removes previous counters on refresh failure while preserving the cached link", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json(summary)).mockResolvedValueOnce(new Response(null, { status: 503 }));
  const { rerender } = render(<ProfileReferralProgram accountId="account-a" />);
  expect(await screen.findByRole("textbox")).toBeInTheDocument();
  rerender(<ProfileReferralProgram accountId="account-a" preview />);
  rerender(<ProfileReferralProgram accountId="account-a" />);
  expect(await screen.findByRole("alert")).toBeInTheDocument();
  expect(screen.queryByText("7")).not.toBeInTheDocument();
  expect(screen.getByRole("textbox")).toHaveValue(`${window.location.origin}/ru/invite/INVITE1234`);
 });
});
