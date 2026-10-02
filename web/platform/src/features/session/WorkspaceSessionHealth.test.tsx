import { act, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
vi.mock("@/i18n/navigation", () => ({ useRouter: () => router }));
const router = { replace: vi.fn(), refresh: vi.fn() };
import { WorkspaceSessionHealth } from "./WorkspaceSessionHealth";
import { accountChangedEvent, sessionRequiredEvent } from "@/lib/web-api/browser-session";
import { endBrowserSession } from "@/lib/web-api/browser-session-state";
afterEach(() => { vi.clearAllMocks(); endBrowserSession(); });
it("hides the private subtree and requests login once after confirmed expiry", () => {
  render(<WorkspaceSessionHealth accountId="account-a" guest={<span>Guest</span>}><span>Private chat</span></WorkspaceSessionHealth>);
  act(() => { window.dispatchEvent(new Event(sessionRequiredEvent)); window.dispatchEvent(new Event(sessionRequiredEvent)); });
  expect(screen.queryByText("Private chat")).not.toBeInTheDocument();
  expect(screen.getByText("Guest")).toBeInTheDocument();
  expect(router.replace).toHaveBeenCalledTimes(1);
});
it("restores the same account after the refreshed server tree confirms it again", () => {
  const first = {};
  const view = render(<WorkspaceSessionHealth accountId="account-a" verification={first} guest={<span>Guest</span>}><span>Private chat</span></WorkspaceSessionHealth>);
  act(() => window.dispatchEvent(new Event(accountChangedEvent)));
  expect(screen.queryByText("Private chat")).not.toBeInTheDocument();
  view.rerender(<WorkspaceSessionHealth accountId="account-a" verification={{}} guest={<span>Guest</span>}><span>Private chat</span></WorkspaceSessionHealth>);
  expect(screen.getByText("Private chat")).toBeInTheDocument();
});
