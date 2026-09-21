import { NextResponse, type NextRequest } from "next/server";

import { returnPathFromURL } from "@/lib/auth/return-path";
import { localeCookieName, resolveLocale } from "@/i18n/locales";
import { isServicePath, localeFromPath, localeRequestHeader, localizeHref, stripLocale } from "@/i18n/routing";

const returnCookieName = "__Host-nh-return-to";

export const config = {
  matcher: ["/((?!_next/|assets/|web/|api/|health$|favicon.ico$|robots.txt$|sitemap.xml$).*)"],
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
  const pathname = request.nextUrl.pathname;
  if (pathname.includes("\\") || pathname.includes("//") || /%(?:2f|5c|25)/i.test(pathname)) {
    return new NextResponse(null, { status: 400, headers: { "X-Robots-Tag": "noindex" } });
  }
  if (isServicePath(pathname)) return NextResponse.next();
  const locale = localeFromPath(pathname);
  const internalPath = stripLocale(pathname);
  if ((!locale && /^\/[a-z]{2}(?:-[a-z]{2})?(?:\/|$)/i.test(pathname))
    || (locale && (isServicePath(internalPath) || localeFromPath(internalPath)))) {
    return new NextResponse(null, { status: 404, headers: { "X-Robots-Tag": "noindex" } });
  }
  if (!locale) {
    const target = request.nextUrl.clone();
    target.pathname = localizeHref(pathname, resolveLocale(request.cookies.get(localeCookieName)?.value));
    const response = NextResponse.redirect(target, 307);
    response.headers.set("Cache-Control", "private, no-store");
    return response;
  }
  const nonce = createNonce();
  const contentSecurityPolicy = createContentSecurityPolicy(nonce);
  const requestHeaders = new Headers(request.headers);
  requestHeaders.set("x-nonce", nonce);
  requestHeaders.set("Content-Security-Policy", contentSecurityPolicy);
  requestHeaders.set(localeRequestHeader, locale);

  const target = request.nextUrl.clone();
  target.pathname = internalPath;
  const returnPath = returnPathFromURL(pathname, request.nextUrl.searchParams);
  const response = NextResponse.rewrite(target, { request: { headers: requestHeaders } });
  response.headers.set("Content-Security-Policy", contentSecurityPolicy);

  const isPrefetch = request.headers.has("next-router-prefetch")
    || request.headers.get("purpose") === "prefetch"
    || request.headers.get("sec-purpose")?.includes("prefetch");
  if (!isPrefetch && (request.method === "GET" || request.method === "HEAD")) {
    response.cookies.set(localeCookieName, locale, {
      httpOnly: true, secure: process.env.NODE_ENV === "production", sameSite: "lax", path: "/", maxAge: 31536000,
    });
  }
  if (returnPath && !isPrefetch && request.method === "GET") {
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
