import { webBrowserMutation } from "@/lib/web-api/browser";

// Acceptance is best effort for authentication and idempotent at the server.
// The HttpOnly intent is cleared only on success or terminal ineligibility.
export async function acceptPendingInvitation(signal: AbortSignal): Promise<boolean> {
  try {
    const response = await webBrowserMutation("/web/v1/referrals/accept", {
      method: "POST", signal: AbortSignal.any([signal, AbortSignal.timeout(5_000)]),
    });
    return response.ok;
  } catch { return false; }
}
