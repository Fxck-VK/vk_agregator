import { cleanup, render } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
vi.mock("@/i18n/navigation", () => ({ useRouter: () => ({ refresh }) }));
vi.mock("@/lib/web-api/browser-session", () => ({ refreshBrowserSession: vi.fn() }));
import { refreshBrowserSession } from "@/lib/web-api/browser-session";
import { HomeSessionRecovery } from "./HomeSessionRecovery";
const refresh = vi.fn();
afterEach(() => { cleanup(); vi.clearAllMocks(); });

it("adds no UI and rechecks the server route only after successful cookie recovery", async () => {
  vi.mocked(refreshBrowserSession).mockResolvedValue(new Response(null, { status: 200 }));
  const { container } = render(<HomeSessionRecovery />);
  await vi.waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
  expect(container).toBeEmptyDOMElement();
});
it.each([401, 403, 429, 503])("leaves the guest landing available after recovery status %s", async status => {
  vi.mocked(refreshBrowserSession).mockResolvedValue(new Response(null, { status }));
  render(<HomeSessionRecovery />); await vi.waitFor(() => expect(refreshBrowserSession).toHaveBeenCalled());
  expect(refresh).not.toHaveBeenCalled();
});
it("ignores recovery completion after navigating away", async () => {
  let done!: (response: Response) => void;
  vi.mocked(refreshBrowserSession).mockReturnValue(new Promise(resolve => { done = resolve; }));
  const { unmount } = render(<HomeSessionRecovery />); unmount();
  done(new Response(null, { status: 200 })); await Promise.resolve();
  expect(refresh).not.toHaveBeenCalled();
});
it("keeps the public landing on network failure", async () => {
  vi.mocked(refreshBrowserSession).mockRejectedValue(new Error("synthetic network failure"));
  render(<HomeSessionRecovery />); await Promise.resolve(); await Promise.resolve();
  expect(refresh).not.toHaveBeenCalled();
});
