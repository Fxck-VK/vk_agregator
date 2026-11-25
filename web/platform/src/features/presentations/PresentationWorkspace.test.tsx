import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { PresentationWorkspace } from "./PresentationWorkspace";

const client = vi.hoisted(() => ({ read: vi.fn() }));
vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: client.read, webBrowserMutation: vi.fn() }));
vi.mock("@/features/account/WorkspaceAccount/WorkspaceAccount", () => ({
  useWorkspaceAccountSnapshot: () => ({ profile: { account_id: "account" } }),
}));
vi.mock("@/components/layout/WorkspacePageFrame/WorkspacePageFrame", () => ({
  WorkspacePageFrame: ({ children }: { children: ReactNode }) => <main>{children}</main>,
}));
vi.mock("@/i18n/LocaleProvider", () => ({
  useLocale: () => "ru",
  useDictionary: () => ({ navigation: { presentations: "Презентации" } }),
}));
vi.mock("@/i18n/Link", () => ({
  default: ({ children, href }: { children: ReactNode; href: string }) => <a href={href}>{children}</a>,
}));
vi.mock("./PresentationStudio", () => ({
  PresentationStudio: ({ initialRun }: { initialRun?: { job: string } }) =>
    <p>{initialRun?.job ?? "Новый бриф"}</p>,
}));

const conversation = "12345678-0000-4000-8000-012345678901";
const job = "12345678-0000-4000-8000-012345678902";
const entry = {
  conversation_id: conversation, job_id: job, title: "Alfa Romeo",
  status: "provider_processing", created_at: "2026-11-25T14:00:00Z", model_name: "Модель",
};

afterEach(() => { cleanup(); vi.resetAllMocks(); vi.useRealTimers(); });

describe("workspace presentation history", () => {
  it("opens the list from the workspace and links each status to the exact job", async () => {
    client.read.mockResolvedValueOnce(Response.json({ items: [entry] }));
    render(<PresentationWorkspace preview={false} />);
    expect(screen.getByRole("link", { name: "Новая презентация" })).toHaveAttribute("href", "/app/presentations?new=1");
    expect(await screen.findByRole("link", { name: /Alfa Romeo/ })).toHaveAttribute(
      "href", `/app/presentations?run=${conversation}&job=${job}`,
    );
    expect(screen.getByText("Модель пишет слайды")).toBeVisible();
  });

  it("refreshes active items and stops polling after completion", async () => {
    vi.useFakeTimers();
    client.read.mockResolvedValueOnce(Response.json({ items: [entry] }));
    client.read.mockResolvedValueOnce(Response.json({ items: [{ ...entry, status: "succeeded" }] }));
    render(<PresentationWorkspace preview={false} />);
    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(screen.getByText("Модель пишет слайды")).toBeVisible();
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(screen.getByText("Готово")).toBeVisible();
    await act(() => vi.advanceTimersByTimeAsync(30_000));
    expect(client.read).toHaveBeenCalledTimes(2);
  });

  it("keeps an explicit route back to the list when reopening a job", () => {
    render(<PresentationWorkspace preview={false} run={{ conversation, job }} />);
    expect(screen.getByRole("link", { name: "Мои презентации" })).toHaveAttribute("href", "/app/presentations");
    expect(screen.getByText(job)).toBeVisible();
    expect(client.read).not.toHaveBeenCalled();
  });

  it("uses the server cursor for older pages and allows returning to the latest page", async () => {
    client.read.mockResolvedValueOnce(Response.json({ items: [{ ...entry, status: "succeeded" }], next_cursor: "opaque" }));
    client.read.mockResolvedValueOnce(Response.json({ items: [{ ...entry, title: "Прежняя тема", status: "failed_terminal" }] }));
    client.read.mockResolvedValueOnce(Response.json({ items: [{ ...entry, status: "succeeded" }], next_cursor: "opaque" }));
    render(<PresentationWorkspace preview={false} />);
    fireEvent.click(await screen.findByRole("button", { name: "Старее" }));
    expect(await screen.findByText("Прежняя тема")).toBeVisible();
    expect(screen.getByText("Ошибка генерации")).toBeVisible();
    expect(client.read.mock.calls[1][0]).toBe("/web/v1/presentations?cursor=opaque");
    fireEvent.click(screen.getByRole("button", { name: "Новее" }));
    expect(await screen.findByText("Alfa Romeo")).toBeVisible();
  });
});
