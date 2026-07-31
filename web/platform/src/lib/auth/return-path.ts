const pathSegment = /^[A-Za-z0-9\-._~!$&'()*+,;=:@]+$/;

export function safeReturnPath(pathname: string): string | null {
  if (pathname === "/app") {
    return pathname;
  }

  if (
    !pathname.startsWith("/app/") ||
    pathname.includes("//") ||
    pathname.includes("\\\\") ||
    pathname.includes("%") ||
    pathname.includes("?") ||
    pathname.includes("#")
  ) {
    return null;
  }

  const segments = pathname.slice("/app/".length).split("/");
  if (
    segments.some(
      (segment) =>
        segment.length === 0 || segment === "." || segment === ".." || !pathSegment.test(segment),
    )
  ) {
    return null;
  }

  return pathname;
}
