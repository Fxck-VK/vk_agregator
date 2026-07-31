import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/headers", () => ({
  cookies: vi.fn(),
}));

vi.mock("@/features/auth/LoginForm/LoginForm", () => ({
  LoginForm: vi.fn(() => <div data-testid="login-form">form</div>),
}));

import { cookies } from "next/headers";

import { LoginForm } from "@/features/auth/LoginForm/LoginForm";
import { metadata } from "./layout";
import LoginPage from "./page";

describe("LoginPage", () => {
  beforeEach(() => {
    vi.mocked(cookies).mockResolvedValue({ get: vi.fn() } as never);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("passes only a validated server-owned return cookie to the account-free form", async () => {
    vi.mocked(cookies).mockResolvedValue({
      get: vi.fn().mockReturnValue({ value: "/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906" }),
    } as never);

    const markup = renderToStaticMarkup(await LoginPage());

    expect(markup).toContain('data-testid="login-form"');
    expect(LoginForm).toHaveBeenCalledWith(
      { returnTo: "/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906" },
      undefined,
    );
    expect(markup).not.toContain("d7c979f5-24e5-4f88-924b-a592d6e5a906");
    expect(metadata.robots).toEqual({ index: false, follow: false });
  });

  it.each([undefined, "https://attacker.example/app", "/app/chat?next=/app"])(
    "does not pass an absent or unsafe return cookie: %s",
    async (cookieValue) => {
      vi.mocked(cookies).mockResolvedValue({
        get: vi.fn().mockReturnValue(cookieValue ? { value: cookieValue } : undefined),
      } as never);

      renderToStaticMarkup(await LoginPage());

      expect(LoginForm).toHaveBeenCalledWith({}, undefined);
    },
  );
});
