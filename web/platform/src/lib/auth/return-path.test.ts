import { describe, expect, it } from "vitest";

import { safeReturnPath } from "./return-path";

describe("safeReturnPath", () => {
  it.each([
    "/app",
    "/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906",
  ])("returns the literal canonical private pathname: %s", (pathname) => {
    expect(safeReturnPath(pathname)).toBe(pathname);
  });

  it.each([
    "",
    "app",
    "/application",
    "/other",
    "//attacker.example/app",
    "https://attacker.example/app",
    "/app//chat",
    "/app/./chat",
    "/app/../chat",
    "/app\\chat",
    "/app/%2fchat",
    "/app/%252fchat",
    "/app/chat?next=/app",
    "/app/chat#fragment",
  ])("rejects an unsafe or non-canonical value: %s", (pathname) => {
    expect(safeReturnPath(pathname)).toBeNull();
  });
});
