import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import { StrictMode } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { WorkspaceAccountProvider, useWorkspaceAccountSnapshot } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import { WorkspaceConversationListProvider, useWorkspaceConversationList } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { webBrowserFetch } from "@/lib/web-api/browser";
vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));
const profile = { account_id: "10000000-0000-4000-8000-000000000001", identity_refs: [] };
function Probe() {
  const balance = useWorkspaceAccountSnapshot();
  const chats = useWorkspaceConversationList();
  return <><span>usable screen</span><span>{balance.balance ?? "balance pending"}</span><span>{chats.failed ? "chats failed" : chats.pending ? "chats pending" : "chats ready"}</span></>;
}
function Workspace({ account = "A" }: { account?: string }) {
  return <WorkspaceAccountProvider key={account} deferred snapshot={{ profile: { ...profile, account_id: account }, balance: null }}>
    <WorkspaceConversationListProvider deferred accountId={account} initialConversations={[]}><Probe /></WorkspaceConversationListProvider>
  </WorkspaceAccountProvider>;
}
afterEach(() => { cleanup(); vi.resetAllMocks(); });
it("loads supplementary resources independently and does not reuse private data across accounts", async () => {
  let finishBalance!: (response: Response) => void;
  vi.mocked(webBrowserFetch).mockImplementation(path => path === "/web/v1/balance"
    ? new Promise(resolve => { finishBalance = resolve; }) : Promise.resolve(new Response(null, { status: 503 })));
  const view = render(<StrictMode><Workspace /></StrictMode>);
  expect(screen.getByText("usable screen")).toBeVisible();
  await screen.findByText("chats failed");
  expect(screen.getByText("balance pending")).toBeVisible();
  await act(async () => finishBalance(Response.json({ balance: 104 })));
  await screen.findByText("104");
  view.rerender(<StrictMode><Workspace /></StrictMode>);
  await act(async () => {});
  expect(screen.getByText("104")).toBeVisible();
  await act(async () => finishBalance(Response.json({ balance: 130 })));
  await screen.findByText("130");
  view.rerender(<StrictMode><Workspace account="B" /></StrictMode>);
  await waitFor(() => expect(screen.queryByText("130")).toBeNull());
  expect(screen.getByText("balance pending")).toBeVisible();
});
