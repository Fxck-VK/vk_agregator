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

function createContentSecurityPolicy(nonce: string): string {
  const developmentScriptSource = process.env.NODE_ENV === "development" ? " 'unsafe-eval'" : "";
  const developmentStyleElements =
    process.env.NODE_ENV === "development" ? "style-src-elem 'self' 'unsafe-inline'" : null;

  return [
    "default-src 'self'",
    `script-src 'self' 'nonce-${nonce}' 'strict-dynamic'${developmentScriptSource}`,
    `style-src 'self' 'nonce-${nonce}'`,
    "style-src-attr 'unsafe-inline'",
    developmentStyleElements,
    "img-src 'self' data: blob:",
    "font-src 'self'",
    "connect-src 'self'",
    "media-src 'self' blob:",
    "object-src 'none'",
    "base-uri 'none'",
    "form-action 'self'",
    "frame-src 'none'",
    "frame-ancestors 'none'",
    "upgrade-insecure-requests",
  ]
    .filter((directive): directive is string => directive !== null)
    .join("; ");
}

export function proxy(request: NextRequest): NextResponse {
  const nonce = createNonce();
  const contentSecurityPolicy = createContentSecurityPolicy(nonce);
  const requestHeaders = new Headers(request.headers);
  requestHeaders.set("x-nonce", nonce);
  requestHeaders.set("Content-Security-Policy", contentSecurityPolicy);

  const returnPath = safeReturnPath(request.nextUrl.pathname);
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
