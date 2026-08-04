import { NextResponse, type NextRequest } from "next/server";

import { safeReturnPath } from "@/lib/auth/return-path";

const returnCookieName = "__Host-nh-return-to";

export const config = {
  matcher: ["/", "/login", "/app/:path*"],
};

function createNonce(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return btoa(String.fromCharCode(...bytes));
}

function createContentSecurityPolicy(nonce?: string): string {
  const isDevelopment = process.env.NODE_ENV === "development";
  const scriptSources = nonce
    ? `script-src 'self' 'nonce-${nonce}' 'strict-dynamic'`
    // The cached public page cannot carry a per-request nonce. Keep this exception scoped to `/`.
    : "script-src 'self' 'unsafe-inline'";
  const styleSources = nonce
    ? `style-src 'self' 'nonce-${nonce}'`
    : "style-src 'self' 'unsafe-inline'";

  return [
    "default-src 'self'",
    `${scriptSources}${isDevelopment ? " 'unsafe-eval'" : ""}`,
    styleSources,
    "img-src 'self' data: blob:",
    "font-src 'self'",
    `connect-src 'self'${isDevelopment ? " ws: wss:" : ""}`,
    "media-src 'self' blob:",
    "object-src 'none'",
    "base-uri 'none'",
    "form-action 'self'",
    "frame-src 'none'",
    "frame-ancestors 'none'",
    "upgrade-insecure-requests",
  ].join("; ");
}

export function proxy(request: NextRequest): NextResponse {
  const isPublicHomepage = request.nextUrl.pathname === "/";
  const nonce = isPublicHomepage ? undefined : createNonce();
  const contentSecurityPolicy = createContentSecurityPolicy(nonce);
  const requestHeaders = new Headers(request.headers);
  if (nonce) {
    requestHeaders.set("x-nonce", nonce);
    requestHeaders.set("Content-Security-Policy", contentSecurityPolicy);
  }

  const requestedNext = request.nextUrl.pathname === "/login"
    ? safeReturnPath(request.nextUrl.searchParams.get("next") ?? "")
    : null;
  const returnPath = requestedNext ?? safeReturnPath(request.nextUrl.pathname);
  const response = NextResponse.next({ request: { headers: requestHeaders } });
  response.headers.set("Content-Security-Policy", contentSecurityPolicy);

  if (returnPath) {
    response.cookies.set({
      name: returnCookieName,
      value: returnPath,
      httpOnly: true,
      secure: true,
      sameSite: "lax",
      path: "/",
      maxAge: 300,
    });
  }

  return response;
}
