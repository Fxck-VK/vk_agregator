import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AccountSecurity } from "./AccountSecurity";
import { previewAuthMethods } from "@/lib/auth/methods";
import { localWorkspacePreviewProfile } from "@/features/session/local-workspace-preview";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { requestWorkspaceLogout } from "@/features/session/WorkspaceLogout/workspace-logout-request";

const navigation = vi.hoisted(() => ({ replace: vi.fn(), refresh: vi.fn() }));
vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn(), webBrowserMutation: vi.fn() }));
vi.mock("@/features/session/WorkspaceLogout/workspace-logout-request", () => ({ requestWorkspaceLogout: vi.fn() }));
vi.mock("server-only", () => ({}));
vi.mock("next/navigation", () => ({ useRouter: () => navigation }));

afterEach(() => { cleanup(); vi.clearAllMocks(); });
describe("AccountSecurity", () => {
  it("ends the live cookie session even when refresh has rotated the listed session ID", async () => {
    const id = "31000000-0000-4000-8000-000000000002";
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ items: [{ id, account_id: localWorkspacePreviewProfile.account_id, created_at: "2026-10-01T12:00:00Z", updated_at: "2026-10-01T12:00:00Z", expires_at: "2026-10-31T12:00:00Z", revoked: false, current: true }] }));
    vi.mocked(requestWorkspaceLogout).mockResolvedValue();
    render(<AccountSecurity profile={localWorkspacePreviewProfile} methods={previewAuthMethods} />);
    await screen.findByText("Текущая сессия");
    fireEvent.click(screen.getByRole("button", { name: "Завершить" }));
    fireEvent.click(screen.getAllByRole("button", { name: "Завершить" }).at(-1)!);
    await waitFor(() => expect(requestWorkspaceLogout).toHaveBeenCalledOnce());
    expect(webBrowserMutation).not.toHaveBeenCalled();
    expect(navigation.replace).toHaveBeenCalledWith("/ru/login");
  });
  it("shows current sessions and revokes only the selected session through the CSRF mutation helper", async () => {
    const id = "31000000-0000-4000-8000-000000000001";
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ items: [{ id, account_id: localWorkspacePreviewProfile.account_id, created_at: "2026-10-01T12:00:00Z", updated_at: "2026-10-01T12:00:00Z", expires_at: "2026-10-31T12:00:00Z", revoked: false, current: false }] }));
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 204 }));
    render(<AccountSecurity profile={localWorkspacePreviewProfile} methods={previewAuthMethods} />);
    await screen.findByText("Другой вход");
    fireEvent.click(screen.getByRole("button", { name: "Завершить" }));
    expect(webBrowserMutation).not.toHaveBeenCalled();
    fireEvent.click(screen.getAllByRole("button", { name: "Завершить" }).at(-1)!);
    await screen.findByText("Других активных сессий нет.");
    expect(webBrowserMutation).toHaveBeenCalledWith(`/web/v1/account/sessions/${id}/revoke`, expect.objectContaining({ method: "POST" }));
  });
  it("does not offer unlinking the last identity and never performs writes in preview", () => {
    render(<AccountSecurity profile={localWorkspacePreviewProfile} methods={previewAuthMethods} preview />);
    expect(screen.getByRole("button", { name: "Отвязать" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Привязать почту" }));
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });
});
