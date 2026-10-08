import { afterEach, expect, it, vi } from "vitest";
import { webBrowserMutation } from "@/lib/web-api/browser";
import { acceptPendingInvitation } from "./attribution";
vi.mock("@/lib/web-api/browser", () => ({ webBrowserMutation: vi.fn() }));
afterEach(() => vi.clearAllMocks());
it("uses the authenticated mutation boundary without account data from the browser", async () => {
 vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 204 }));
 expect(await acceptPendingInvitation(new AbortController().signal)).toBe(true);
 expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/referrals/accept", expect.objectContaining({ method: "POST" }));
 expect(vi.mocked(webBrowserMutation).mock.calls[0][1].body).toBeUndefined();
});
it("returns a retry result for an outage without breaking successful login", async () => {
 vi.mocked(webBrowserMutation).mockRejectedValue(new TypeError("offline"));
 expect(await acceptPendingInvitation(new AbortController().signal)).toBe(false);
});
