import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/headers", () => ({
  cookies: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(() => { throw new Error("NEXT_REDIRECT"); }),
}));

vi.mock("@/lib/web-api/server", () => ({
  webServerFetch: vi.fn(),
}));

vi.mock("@/features/auth/LoginForm/LoginForm", () => ({
  LoginForm: vi.fn(() => <div data-testid="login-form">form</div>),
}));

import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { LoginForm } from "@/features/auth/LoginForm/LoginForm";
import { webServerFetch } from "@/lib/web-api/server";
import { metadata } from "./layout";
import LoginPage from "./page";

describe("LoginPage", () => {
  beforeEach(() => {
    vi.mocked(cookies).mockResolvedValue({ get: vi.fn(), has: vi.fn().mockReturnValue(false) } as never);
    vi.mocked(webServerFetch).mockResolvedValue(new Response(null, { status: 401 }));
  });

  it("bypasses the form for an existing authenticated session and keeps the selected target", async () => {
    vi.mocked(cookies).mockResolvedValue({
      get: vi.fn().mockReturnValue({ value: "/app/files" }),
      has: vi.fn().mockReturnValue(false),
    } as never);
    vi.mocked(webServerFetch).mockResolvedValueOnce(new Response(null, { status: 200 }));

    await expect(LoginPage()).rejects.toThrow("NEXT_REDIRECT");

    expect(redirect).toHaveBeenCalledWith("/app/files");
    expect(LoginForm).not.toHaveBeenCalled();
  });

  it("continues through the app refresh flow when only a refresh cookie remains", async () => {
    vi.mocked(cookies).mockResolvedValue({
      get: vi.fn().mockReturnValue({ value: "/app/chats" }),
      has: vi.fn().mockImplementation((name: string) => name === "nh_refresh"),
    } as never);

    await expect(LoginPage()).rejects.toThrow("NEXT_REDIRECT");

    expect(redirect).toHaveBeenCalledWith("/app/chats");
  });

  it("keeps the login form available after the app refresh flow already failed", async () => {
    vi.mocked(cookies).mockResolvedValue({
      get: vi.fn().mockReturnValue({ value: "/app/chats" }),
      has: vi.fn().mockImplementation((name: string) => name === "nh_refresh"),
    } as never);

    const markup = renderToStaticMarkup(
      await LoginPage({ searchParams: Promise.resolve({ refresh_failed: "1" }) }),
    );

    expect(markup).toContain('data-testid="login-form"');
    expect(redirect).not.toHaveBeenCalled();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("passes only a validated server-owned return cookie to the account-free form", async () => {
    vi.mocked(cookies).mockResolvedValue({
      get: vi.fn().mockReturnValue({ value: "/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906" }),
      has: vi.fn().mockReturnValue(false),
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
        has: vi.fn().mockReturnValue(false),
      } as never);

      renderToStaticMarkup(await LoginPage());

      expect(LoginForm).toHaveBeenCalledWith({}, undefined);
    },
  );
});
