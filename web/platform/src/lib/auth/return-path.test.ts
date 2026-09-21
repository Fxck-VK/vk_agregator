import { describe, expect, it } from "vitest";

import { safeReturnPath } from "./return-path";

describe("safeReturnPath", () => {
  it.each([
    "/app",
    "/en/app",
    "/ru/app/files?category=images",
    "/en/app/chats?model=gpt_image_2_5_flare",
    "/en/app/payment-return?payment_id=90000000-0000-4000-8000-000000000001",
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
    "/fr/app",
    "/en/ru/app",
    "/en/app?next=https://attacker.example",
    "/en/app?model=x&model=y",
    "/en/app?model=%2f%2fattacker.example",
    "/en/app?prompt=private-text",
  ])("rejects an unsafe or non-canonical value: %s", (pathname) => {
    expect(safeReturnPath(pathname)).toBeNull();
  });
});
