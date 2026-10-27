import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));
vi.mock("@/lib/auth/sign-in-navigation", () => ({ replaceSignInDocument: vi.fn() }));
import { webBrowserFetch } from "@/lib/web-api/browser";
import { replaceSignInDocument } from "@/lib/auth/sign-in-navigation";
import { ru } from "@/i18n/ru";
import type { AuthMethods } from "@/lib/auth/methods";
import { LoginForm } from "./LoginForm";

const replace = vi.mocked(replaceSignInDocument);
const profile = { account_id: "10000000-0000-4000-8000-000000000001", identity_refs: [], password_set: true };
const methods: AuthMethods = { registration: true, password: true, recovery: true, email_link: true, phone_link: false, providers: [] };
const resetBinding = "c".repeat(64);
function credentials(email = "member@example.test", password = "synthetic-password") {
  fireEvent.change(screen.getByLabelText(ru.login.emailLabel), { target: { value: email } });
  fireEvent.change(screen.getByLabelText(ru.login.passwordLabel), { target: { value: password } });
}
function mutations() { return vi.mocked(webBrowserFetch).mock.calls.filter(([, options]) => options?.method === "POST"); }
beforeEach(() => {
  window.history.replaceState({}, "", "/ru/login");
  vi.mocked(webBrowserFetch).mockReset().mockImplementation(async path => path === "/web/v1/auth/remembered"
    ? Response.json({ email_hint: "" }) : path === "/web/v1/auth/password/reset-status"
      ? Response.json({ binding: resetBinding }) : new Response(null, { status: 204 }));
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(profile)));
});
afterEach(() => { cleanup(); vi.clearAllMocks(); vi.unstubAllGlobals(); window.history.replaceState({}, "", "/"); });

describe("canonical LoginForm", () => {
  it("starts with configured providers and inline email without submitting credentials", () => {
    render(<LoginForm entry="choice" methods={{ ...methods, providers: ["vk", "yandex"] }} />);
    expect(screen.getByRole("heading", { name: "Добро пожаловать в Epifera" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Войти с Яндекс ID" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Войти с VK ID" })).toBeEnabled();
    expect(screen.queryByLabelText(ru.login.passwordLabel)).not.toBeInTheDocument();
    const next = screen.getByRole("button", { name: "Продолжить" }); expect(next).toBeDisabled();
    fireEvent.change(screen.getByLabelText(ru.login.emailLabel), { target: { value: "owner@example.test" } });
    fireEvent.submit(next.closest("form")!);
    expect(screen.getByLabelText(ru.login.emailLabel)).toHaveValue("owner@example.test");
    expect(screen.getByLabelText(ru.login.passwordLabel)).toBeInTheDocument();
    expect(mutations()).toHaveLength(0);
  });
  it("clears the unsubmitted password when leaving email sign-in", () => {
    render(<LoginForm entry="choice" methods={methods} />);
    fireEvent.change(screen.getByLabelText(ru.login.emailLabel), { target: { value: "owner@example.test" } });
    fireEvent.click(screen.getByRole("button", { name: "Продолжить" }));
    fireEvent.change(screen.getByLabelText(ru.login.passwordLabel), { target: { value: "unsubmitted-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Другие способы входа" }));
    expect(screen.getByRole("heading", { name: "Добро пожаловать в Epifera" })).toHaveFocus();
    fireEvent.click(screen.getByRole("button", { name: "Продолжить" }));
    expect(screen.getByLabelText(ru.login.passwordLabel)).toHaveValue(""); expect(mutations()).toHaveLength(0);
  });
  it("opens email/password registration directly with the chosen email and no request", () => {
    render(<LoginForm entry="choice" preview methods={methods} />);
    fireEvent.change(screen.getByLabelText(ru.login.emailLabel), { target: { value: "new-owner@example.test" } });
    fireEvent.click(screen.getByRole("button", { name: "Зарегистрироваться" }));
    expect(screen.getByRole("heading", { name: "Создать аккаунт" })).toBeInTheDocument();
    expect(screen.getByLabelText(ru.login.emailLabel)).toHaveValue("new-owner@example.test");
    expect(screen.getByLabelText(ru.login.passwordLabel)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Зарегистрироваться" })).toBeDisabled();
    expect(webBrowserFetch).not.toHaveBeenCalled();
  });
  it("keeps unavailable login methods unavailable while enabled registration stays independent", () => {
    render(<LoginForm entry="choice" methods={{ ...methods, password: false, recovery: false, providers: ["google"] }} />);
    expect(screen.queryByRole("button", { name: /Google|Яндекс|VK ID/ })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Продолжить" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Зарегистрироваться" }));
    expect(screen.getByLabelText(ru.login.passwordLabel)).toBeInTheDocument(); expect(mutations()).toHaveLength(0);
  });
  it("does not expose a registration action when the backend disables it", () => {
    render(<LoginForm entry="choice" methods={{ ...methods, registration: false }} />);
    expect(screen.queryByRole("button", { name: "Зарегистрироваться" })).not.toBeInTheDocument();
  });
  it("offers Yandex only when enabled by the backend", () => {
    const { rerender } = render(<LoginForm entry="choice" methods={{ ...methods, providers: ["yandex"] }} />);
    expect(screen.getByRole("button", { name: "Войти с Яндекс ID" })).toBeEnabled();
    rerender(<LoginForm entry="choice" methods={methods} />);
    expect(screen.queryByRole("button", { name: "Войти с Яндекс ID" })).not.toBeInTheDocument();
  });
  it("rejects an untrusted OAuth URL returned by server start", async () => {
    vi.mocked(webBrowserFetch).mockImplementation(async path => path === "/web/v1/auth/remembered" ? Response.json({ email_hint: "" }) : Response.json({ authorization_url: "https://untrusted.example.test/" }));
    render(<LoginForm entry="choice" methods={{ ...methods, providers: ["yandex"] }} />);
    fireEvent.click(screen.getByRole("button", { name: "Войти с Яндекс ID" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(ru.auth.providerFailure);
    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/auth/oauth/yandex/start", expect.objectContaining({ method: "POST", body: JSON.stringify({ locale: "ru" }) }));
  });
  it("hides stale Google and prevents provider mutations in preview", () => {
    render(<LoginForm entry="choice" preview methods={{ ...methods, providers: ["google", "yandex", "vk"] }} />);
    expect(screen.queryByRole("button", { name: /Google/ })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Войти с Яндекс ID" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Войти с Яндекс ID" })); expect(webBrowserFetch).not.toHaveBeenCalled();
  });
  it("keeps the complete password and verification preview free of account requests", async () => {
    render(<LoginForm preview methods={methods} />); credentials();
    fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    fireEvent.paste(await screen.findByRole("textbox", { name: "Код из письма: 1" }), { clipboardData: { getData: () => "123456" } });
    fireEvent.click(screen.getByRole("button", { name: "Подтвердить почту" }));
    await screen.findByText(ru.auth.loginPreviewComplete);
    expect(webBrowserFetch).not.toHaveBeenCalled(); expect(replace).not.toHaveBeenCalled();
  });
  it("keeps progress visible and prevents duplicate submits until cookie confirmation", async () => {
    let confirm!: (response: Response) => void;
    vi.stubGlobal("fetch", vi.fn().mockImplementation(() => new Promise<Response>(resolve => { confirm = resolve; })));
    render(<LoginForm />); credentials();
    const form = screen.getByRole("button", { name: "Войти" }).closest("form")!;
    fireEvent.submit(form); fireEvent.submit(form);
    expect(await screen.findByRole("button", { name: "Проверяем вход…" })).toBeDisabled();
    expect(mutations()).toHaveLength(1); expect(replace).not.toHaveBeenCalled();
    confirm(Response.json(profile));
    expect(await screen.findByRole("button", { name: "Открываем рабочее пространство…" })).toBeDisabled();
    expect(replace).toHaveBeenCalledWith("/ru/app");
  });
  it("retries cookie confirmation without resubmitting the password", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response(null, { status: 503 })).mockResolvedValueOnce(Response.json(profile)));
    render(<LoginForm />); credentials(); fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(ru.auth.loginUnavailable);
    expect(screen.queryByLabelText(ru.login.passwordLabel)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Повторить вход" }));
    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith("/ru/app")); expect(mutations()).toHaveLength(1);
  });
  it("does not navigate when accepted credentials fail cookie confirmation", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 401, headers: { "X-Request-ID": "40000000-0000-4000-8000-000000000001" } })));
    render(<LoginForm />); credentials(); fireEvent.click(screen.getByRole("button", { name: "Войти" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Не удалось подтвердить вход.");
    expect(screen.getByRole("alert")).toHaveTextContent("Номер ошибки: 40000000-0000-4000-8000-000000000001");
    expect(replace).not.toHaveBeenCalled();
  });
  it("offers primary or backup email recovery without revealing whether an account exists", async () => {
    render(<LoginForm methods={methods} />); fireEvent.click(screen.getByRole("button", { name: "Забыли пароль?" }));
    expect(screen.getByText(ru.auth.recoveryLinkDescription)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(ru.login.emailLabel), { target: { value: "backup@example.test" } });
    fireEvent.click(screen.getByRole("button", { name: "Отправить ссылку" })); await screen.findByRole("heading", { name: "Проверьте почту" });
    expect(screen.getByText(ru.auth.recoveryLinkSent)).toBeInTheDocument();
    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/auth/password/request-link", expect.objectContaining({ body: JSON.stringify({ email: "backup@example.test", locale: "ru" }) }));
    expect(replace).not.toHaveBeenCalled();
  });
  it("does not consume the reset receipt for an overlong multibyte password", async () => {
    render(<LoginForm methods={methods} initialScreen="reset" />);
    fireEvent.change(await screen.findByLabelText("Новый пароль"), { target: { value: "я".repeat(129) } });
    fireEvent.change(screen.getByLabelText("Повторите пароль"), { target: { value: "я".repeat(129) } });
    expect(screen.getByText(ru.auth.passwordTooLong)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Сохранить пароль" })).toBeDisabled(); expect(mutations()).toHaveLength(0);
    expect(window.history.state.epiferaResetBinding).toBe(resetBinding);
  });
  it("resets through a prepared receipt and returns to sign-in", async () => {
    render(<LoginForm methods={methods} initialScreen="reset" />);
    fireEvent.change(await screen.findByLabelText("Новый пароль"), { target: { value: "replacement-password" } });
    fireEvent.change(screen.getByLabelText("Повторите пароль"), { target: { value: "replacement-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" })); await screen.findByText(ru.auth.resetSuccess);
    expect(webBrowserFetch).toHaveBeenLastCalledWith("/web/v1/auth/password/reset-link", expect.objectContaining({ body: JSON.stringify({ new_password: "replacement-password" }), headers: { "Content-Type": "application/json", "X-NeiroHub-Reset-Binding": resetBinding } }));
    expect(window.history.state.epiferaResetBinding).toBeUndefined();
    fireEvent.click(screen.getByRole("button", { name: "Вернуться ко входу" }));
    expect(screen.getByLabelText(ru.login.passwordLabel)).toHaveValue(""); expect(replace).not.toHaveBeenCalled();
  });
  it("sends JSON credentials with persistence and follows only a safe private return path", async () => {
    render(<LoginForm returnTo="/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906" />); credentials("member@example.test", "secret-password");
    fireEvent.submit(screen.getByRole("button", { name: ru.login.submitLabel }).closest("form")!);
    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith("/ru/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906"));
    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/auth/password/login", expect.objectContaining({ method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email: "member@example.test", password: "secret-password", persistent: false }) }));
  });
  it.each([undefined, "https://attacker.example/app", "/app/chat?next=/app"])("falls back for absent or unsafe return path: %s", async returnTo => {
    render(<LoginForm returnTo={returnTo} />); credentials(); fireEvent.submit(screen.getByRole("button", { name: ru.login.submitLabel }).closest("form")!);
    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith("/ru/app"));
  });
  it.each([
    ["a rejected request", () => Promise.reject(new Error("untrusted backend detail")), ru.auth.loginNetwork],
    ["a non-success response", () => Promise.resolve(new Response(null, { status: 401 })), ru.login.failure],
  ])("shows safe feedback and clears the password after %s", async (_caseName, request, message) => {
    vi.mocked(webBrowserFetch).mockImplementationOnce(request); render(<LoginForm />); credentials();
    const password = screen.getByLabelText(ru.login.passwordLabel);
    fireEvent.submit(screen.getByRole("button", { name: ru.login.submitLabel }).closest("form")!);
    expect(await screen.findByRole("alert")).toHaveTextContent(message); expect(password).toHaveValue("");
    expect(replace).not.toHaveBeenCalled(); expect(screen.queryByText("untrusted backend detail")).not.toBeInTheDocument();
  });
});
