import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(),
}));

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserFetch: vi.fn(),
}));

import { useRouter } from "next/navigation";

import { webBrowserFetch } from "@/lib/web-api/browser";
import { ru } from "@/i18n/ru";

import { LoginForm } from "./LoginForm";

const replace = vi.fn();

describe("LoginForm", () => {
  beforeEach(() => {
    vi.mocked(useRouter).mockReturnValue({ replace } as never);
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("submits only JSON email and password then replaces to a safe private return path after a 2xx response", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(new Response(null, { status: 204 }));
    render(<LoginForm returnTo="/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906" />);

    fireEvent.change(screen.getByLabelText(ru.login.emailLabel), {
      target: { value: "member@example.com" },
    });
    fireEvent.change(screen.getByLabelText(ru.login.passwordLabel), {
      target: { value: "secret-password" },
    });
    fireEvent.submit(screen.getByRole("button", { name: ru.login.submitLabel }).closest("form")!);

    await vi.waitFor(() =>
      expect(replace).toHaveBeenCalledWith("/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906"),
    );
    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/auth/password/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email: "member@example.com", password: "secret-password" }),
    });
  });

  it.each([undefined, "https://attacker.example/app", "/app/chat?next=/app"])(
    "falls back to the workspace root after a 2xx response when returnTo is absent or unsafe: %s",
    async (returnTo) => {
      vi.mocked(webBrowserFetch).mockResolvedValue(new Response(null, { status: 204 }));
      render(<LoginForm returnTo={returnTo} />);

      fireEvent.change(screen.getByLabelText(ru.login.emailLabel), {
        target: { value: "member@example.com" },
      });
      fireEvent.change(screen.getByLabelText(ru.login.passwordLabel), {
        target: { value: "secret-password" },
      });
      fireEvent.submit(screen.getByRole("button", { name: ru.login.submitLabel }).closest("form")!);

      await vi.waitFor(() => expect(replace).toHaveBeenCalledWith("/app"));
    },
  );

  it.each([
    ["a rejected request", () => Promise.reject(new Error("untrusted backend detail"))],
    ["a non-success response", () => Promise.resolve(new Response(null, { status: 401 }))],
  ])("shows only neutral feedback and clears the password after %s", async (_caseName, request) => {
    vi.mocked(webBrowserFetch).mockImplementationOnce(request);
    render(<LoginForm />);

    fireEvent.change(screen.getByLabelText(ru.login.emailLabel), {
      target: { value: "member@example.com" },
    });
    const password = screen.getByLabelText(ru.login.passwordLabel);
    fireEvent.change(password, { target: { value: "secret-password" } });
    fireEvent.submit(screen.getByRole("button", { name: ru.login.submitLabel }).closest("form")!);

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.login.failure);
    expect(password).toHaveValue("");
    expect(replace).not.toHaveBeenCalled();
    expect(screen.queryByText("untrusted backend detail")).not.toBeInTheDocument();
  });
});
