import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(),
}));

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserFetch: vi.fn(),
}));
vi.mock("@/lib/auth/sign-in-navigation", () => ({ replaceSignInDocument: vi.fn() }));

import { useRouter } from "next/navigation";

import { webBrowserFetch } from "@/lib/web-api/browser";
import { replaceSignInDocument } from "@/lib/auth/sign-in-navigation";
import { ru } from "@/i18n/ru";

import { LoginForm } from "./LoginForm";

const replace = vi.mocked(replaceSignInDocument);
const profile = { account_id: "10000000-0000-4000-8000-000000000001", identity_refs: [], password_set: true };

describe("LoginForm", () => {
  it("keeps visible progress and blocks duplicate submits until cookie confirmation and document navigation", async () => {
    let confirm!: (response: Response) => void;
    vi.mocked(webBrowserFetch).mockResolvedValue(new Response(null, { status: 201 }));
    vi.stubGlobal("fetch", vi.fn().mockImplementation(() => new Promise<Response>(resolve => { confirm = resolve; })));
    render(<LoginForm />);
    fireEvent.change(screen.getByLabelText(ru.login.emailLabel), { target: { value: "member@example.test" } });
    fireEvent.change(screen.getByLabelText(ru.login.passwordLabel), { target: { value: "synthetic-password" } });
    const form = screen.getByRole("button", { name: "Войти" }).closest("form")!;
    fireEvent.submit(form); fireEvent.submit(form);
    expect(await screen.findByRole("button", { name: "Проверяем вход…" })).toBeDisabled();
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);
    expect(replace).not.toHaveBeenCalled();
    confirm(Response.json(profile));
    expect(await screen.findByRole("button", { name: "Открываем рабочее пространство…" })).toBeDisabled();
    expect(replace).toHaveBeenCalledWith("/ru/app");
  });
  it("retries session confirmation without submitting the password again", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(new Response(null, { status: 201 }));
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response(null, { status: 503 })).mockResolvedValueOnce(Response.json(profile)));
    render(<LoginForm />);
    fireEvent.change(screen.getByLabelText(ru.login.emailLabel), { target: { value: "member@example.test" } });
    fireEvent.change(screen.getByLabelText(ru.login.passwordLabel), { target: { value: "synthetic-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(ru.auth.loginUnavailable);
    expect(screen.queryByLabelText(ru.login.passwordLabel)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Повторить вход" }));
    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith("/ru/app"));
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);
  });
  it("does not leave the login screen when the server accepts credentials but the browser session is not confirmed", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(new Response(null, { status: 201 }));
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 401, headers: { "X-Request-ID": "40000000-0000-4000-8000-000000000001" } })));
    render(<LoginForm />);
    fireEvent.change(screen.getByLabelText(ru.login.emailLabel), { target: { value: "member@example.test" } });
    fireEvent.change(screen.getByLabelText(ru.login.passwordLabel), { target: { value: "synthetic-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Не удалось подтвердить вход.");
    expect(screen.getByRole("alert")).toHaveTextContent("Номер ошибки: 40000000-0000-4000-8000-000000000001");
    expect(replace).not.toHaveBeenCalled();
  });
  it("offers recovery through the primary or backup email and accepts the backup address", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(new Response(null, { status: 202 }));
    render(<LoginForm methods={{ registration: false, password: true, recovery: true, email_link: true, phone_link: false, providers: [] }} />);
    fireEvent.click(screen.getByRole("button", { name: "Забыли пароль?" }));
    expect(screen.getByText("Введите основную или резервную почту, подтверждённую в аккаунте.")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(ru.login.emailLabel), { target: { value: "backup@example.test" } });
    fireEvent.click(screen.getByRole("button", { name: "Отправить код" }));
    await screen.findByLabelText("Код из письма");
    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/auth/password/request-reset", expect.objectContaining({ body: JSON.stringify({ email: "backup@example.test" }) }));
    fireEvent.change(screen.getByLabelText("Код из письма"), { target: { value: "123456" } });
    fireEvent.change(screen.getByLabelText("Новый пароль"), { target: { value: "replacement-password" } });
    vi.mocked(webBrowserFetch).mockResolvedValue(new Response(null, { status: 204 }));
    fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" }));
    await screen.findByText("Пароль обновлён. Войдите с новым паролем.");
    expect(webBrowserFetch).toHaveBeenLastCalledWith("/web/v1/auth/password/reset", expect.objectContaining({ body: JSON.stringify({ email: "backup@example.test", code: "123456", new_password: "replacement-password" }) }));
  });
  it("does not consume the recovery code when a multibyte password exceeds the server limit", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(new Response(null, { status: 202 }));
    render(<LoginForm methods={{ registration: false, password: true, recovery: true, email_link: true, phone_link: false, providers: [] }} />);
    fireEvent.click(screen.getByRole("button", { name: "Забыли пароль?" }));
    fireEvent.change(screen.getByLabelText(ru.login.emailLabel), { target: { value: "member@example.test" } });
    fireEvent.click(screen.getByRole("button", { name: "Отправить код" }));
    await screen.findByLabelText("Код из письма");
    fireEvent.change(screen.getByLabelText("Код из письма"), { target: { value: "123456" } });
    fireEvent.change(screen.getByLabelText("Новый пароль"), { target: { value: "я".repeat(129) } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Пароль слишком длинный. Используйте более короткий.");
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);
    expect(screen.getByLabelText("Код из письма")).toHaveValue("123456");
  });
  it("requests a recovery code without revealing whether the email exists, then resets with code and new password", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(new Response(null, { status: 202 }));
    render(<LoginForm methods={{ registration: false, password: true, recovery: true, email_link: true, phone_link: false, providers: [] }} />);
    fireEvent.click(screen.getByRole("button", { name: "Забыли пароль?" }));
    fireEvent.change(screen.getByLabelText(ru.login.emailLabel), { target: { value: "member@example.com" } });
    fireEvent.click(screen.getByRole("button", { name: "Отправить код" }));
    await screen.findByLabelText("Код из письма");
    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/auth/password/request-reset", expect.objectContaining({ body: JSON.stringify({ email: "member@example.com" }) }));
    expect(screen.getByText(/Если эта почта привязана/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Код из письма"), { target: { value: "123456" } });
    fireEvent.change(screen.getByLabelText("Новый пароль"), { target: { value: "new-password" } });
    vi.mocked(webBrowserFetch).mockResolvedValue(new Response(null, { status: 204 }));
    fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" }));
    await screen.findByText("Пароль обновлён. Войдите с новым паролем.");
    expect(webBrowserFetch).toHaveBeenLastCalledWith("/web/v1/auth/password/reset", expect.objectContaining({ body: JSON.stringify({ email: "member@example.com", code: "123456", new_password: "new-password" }) }));
    expect(replace).not.toHaveBeenCalled();
  });
  beforeEach(() => {
    vi.mocked(useRouter).mockReturnValue({ replace: vi.fn() } as never);
    vi.stubGlobal("fetch", vi.fn().mockImplementation(async () => Response.json(profile)));
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
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
      expect(replace).toHaveBeenCalledWith("/ru/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906"),
    );
    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/auth/password/login", expect.objectContaining({
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email: "member@example.com", password: "secret-password" }),
    }));
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

      await vi.waitFor(() => expect(replace).toHaveBeenCalledWith("/ru/app"));
    },
  );

  it.each([
    ["a rejected request", () => Promise.reject(new Error("untrusted backend detail")), ru.auth.loginNetwork],
    ["a non-success response", () => Promise.resolve(new Response(null, { status: 401 })), ru.login.failure],
  ])("shows only safe feedback and clears the password after %s", async (_caseName, request, message) => {
    vi.mocked(webBrowserFetch).mockImplementationOnce(request);
    render(<LoginForm />);

    fireEvent.change(screen.getByLabelText(ru.login.emailLabel), {
      target: { value: "member@example.com" },
    });
    const password = screen.getByLabelText(ru.login.passwordLabel);
    fireEvent.change(password, { target: { value: "secret-password" } });
    fireEvent.submit(screen.getByRole("button", { name: ru.login.submitLabel }).closest("form")!);

    expect(await screen.findByRole("alert")).toHaveTextContent(message);
    expect(password).toHaveValue("");
    expect(replace).not.toHaveBeenCalled();
    expect(screen.queryByText("untrusted backend detail")).not.toBeInTheDocument();
  });
});
