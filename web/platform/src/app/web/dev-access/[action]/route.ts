import { getDevAccessConfig, hasDevAccess, isDevAccessIssuer, issueDevAccessCookie, safeDevReturnPath } from "@/lib/dev-access/session";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

const privateHeaders = { "Cache-Control": "private, no-store", "X-Robots-Tag": "noindex, nofollow" };

// Nginx blocks public access to this route tree. Only its internal cookie check,
// denial handler and Basic-authenticated login location may reach these actions.
export async function GET(request: Request, context: { params: Promise<{ action: string }> }): Promise<Response> {
  const config = getDevAccessConfig();
  if (!config) return new Response(null, { status: 404, headers: privateHeaders });
  const { action } = await context.params;
  if (action === "check") {
    return new Response(null, { status: hasDevAccess(config, request.headers.get("cookie")) ? 204 : 401, headers: privateHeaders });
  }
  if (action === "issue") {
    if (!isDevAccessIssuer(config, request.headers.get("x-dev-web-proof"), request.headers.get("x-dev-web-user"))) {
      return new Response(null, { status: 403, headers: privateHeaders });
    }
    return new Response(null, {
      status: 303,
      headers: {
        ...privateHeaders,
        "Set-Cookie": issueDevAccessCookie(config),
        Location: safeDevReturnPath(new URL(request.url).searchParams.get("returnTo")),
      },
    });
  }
  if (action === "required") {
    const originalMethod = request.headers.get("x-dev-web-original-method") ?? request.method;
    if (originalMethod === "GET" && request.headers.get("sec-fetch-mode") === "navigate"
      && request.headers.get("sec-fetch-dest") === "document") {
      const returnTo = safeDevReturnPath(request.headers.get("x-dev-web-return-to"));
      return new Response(null, { status: 303, headers: { ...privateHeaders, Location: `/__dev/login?returnTo=${encodeURIComponent(returnTo)}` } });
    }
    // No WWW-Authenticate: fetches, image/video requests and account-session
    // failures must never open a native browser password dialog.
    return Response.json({ error: { code: "DEV_ACCESS_REQUIRED", message: "DEV access has expired. Reload the page to sign in." } }, { status: 401, headers: { ...privateHeaders, "X-NeiroHub-Dev-Access": "required" } });
  }
  return new Response(null, { status: 404, headers: privateHeaders });
}
