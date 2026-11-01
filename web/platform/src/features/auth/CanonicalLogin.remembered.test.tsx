import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));
vi.mock("@/lib/auth/sign-in-navigation", () => ({ replaceSignInDocument: vi.fn() }));
import { webBrowserFetch } from "@/lib/web-api/browser";
import { replaceSignInDocument } from "@/lib/auth/sign-in-navigation";
import type { AuthMethods } from "@/lib/auth/methods";
import { LoginForm } from "./CanonicalLogin";

const methods: AuthMethods = { registration: true, password: true, recovery: true, email_link: true, phone_link: false, providers: ["vk", "yandex"] };
const hint = { provider: "yandex", email_hint: "Тестовый пользователь" };
function respond(remembered: unknown = hint) {
  vi.mocked(webBrowserFetch).mockImplementation(async path => path === "/web/v1/auth/remembered"
    ? Response.json(remembered) : new Response(null, { status: 204 }));
}
function mutations() { return vi.mocked(webBrowserFetch).mock.calls.filter(([, options]) => options?.method === "POST"); }
beforeEach(() => {
  window.history.replaceState(null, "", "/ru/login");
  vi.mocked(webBrowserFetch).mockReset(); respond();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ account_id: "10000000-0000-4000-8000-000000000001", identity_refs: [] })));
});
afterEach(() => { cleanup(); vi.clearAllMocks(); vi.unstubAllGlobals(); });

it("personalizes the first provider inside the common form, keeping all alternatives", async () => {
  render(<LoginForm methods={methods} entry="choice" />);
  const personalized = await screen.findByRole("button", { name: /Продолжить как Тестовый пользователь.*Яндекс ID/ });
  expect(screen.getByRole("heading", { name: "Добро пожаловать в Epifera" })).toBeInTheDocument();
  expect(screen.queryByRole("heading", { name: "С возвращением" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Войти с VK ID" })).toBeEnabled();
  expect(screen.queryByRole("button", { name: "Войти с Яндекс ID" })).not.toBeInTheDocument();
  expect(screen.getByLabelText("Электронная почта")).toHaveValue("");
  expect(screen.getByRole("button", { name: "Зарегистрироваться" })).toBeEnabled();
  expect(screen.queryByRole("button", { name: "Использовать другой аккаунт" })).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Выбрать аккаунт" }));
  expect(screen.getByRole("menuitem", { name: "Использовать другой аккаунт" })).toBeInTheDocument();
  fireEvent.keyDown(document, { key: "Escape" });
  expect(screen.getAllByRole("button")[0]).toBe(personalized);
  expect(personalized.querySelector('[data-icon="account-avatar"]')).toBeInTheDocument();
  expect(within(personalized).queryByRole("img", { name: /Epifera/ })).not.toBeInTheDocument();
  expect(mutations()).toHaveLength(0);
});

it("starts the actual remembered provider without sending the display hint as identity", async () => {
  const assign = vi.fn(); const originalWindow = window;
  vi.stubGlobal("window", new Proxy(originalWindow, { get(target, key) { return key === "location" ? { assign, href: originalWindow.location.href } : Reflect.get(target, key, target); } }));
  vi.mocked(webBrowserFetch).mockImplementation(async path => Response.json(path === "/web/v1/auth/remembered" ? hint : { authorization_url: "https://oauth.yandex.ru/authorize?client_id=synthetic" }));
  render(<LoginForm methods={methods} entry="choice" />);
  fireEvent.click(await screen.findByRole("button", { name: /Продолжить как/ }));
  await vi.waitFor(() => expect(assign).toHaveBeenCalledWith("https://oauth.yandex.ru/authorize?client_id=synthetic"));
  expect(mutations()).toHaveLength(1);
  expect(webBrowserFetch).toHaveBeenLastCalledWith("/web/v1/auth/oauth/yandex/start", expect.objectContaining({ body: JSON.stringify({ locale: "ru" }) }));
});

it("lets a different typed email take the ordinary password path without forgetting the provider", async () => {
  render(<LoginForm methods={methods} entry="choice" />);
  await screen.findByRole("button", { name: /Продолжить как/ });
  fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "different@example.test" } });
  fireEvent.click(screen.getByRole("button", { name: "Продолжить" }));
  fireEvent.change(screen.getByLabelText("Пароль"), { target: { value: "synthetic-password" } });
  fireEvent.click(screen.getByRole("button", { name: "Войти" }));
  await vi.waitFor(() => expect(replaceSignInDocument).toHaveBeenCalled());
  expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/auth/password/login", expect.objectContaining({ body: JSON.stringify({ email: "different@example.test", password: "synthetic-password", persistent: false }) }));
  expect(mutations()).toHaveLength(1);
});

it("keeps ordinary email entry without an extra remembered-email button and still requires server code", async () => {
  respond({ email_hint: "m***@example.test" });
  render(<LoginForm methods={methods} entry="choice" navigationEnabled />);
  await act(async () => {});
  expect(screen.queryByRole("button", { name: /Продолжить как/ })).not.toBeInTheDocument();
  expect(screen.getByLabelText("Электронная почта")).toHaveValue("");
  fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "different@example.test" } });
  fireEvent.click(screen.getByRole("button", { name: "Продолжить" }));
  expect(screen.getByLabelText("Пароль")).toHaveValue("");
  expect(screen.getByLabelText("Электронная почта")).toHaveValue("different@example.test");
  expect(screen.getByRole("button", { name: "Войти" })).toBeDisabled();
  expect(mutations()).toHaveLength(0);
  expect(window.location.href).not.toContain("example.test");
  fireEvent.change(screen.getByLabelText("Пароль"), { target: { value: "synthetic-password" } });
  fireEvent.click(screen.getByRole("checkbox", { name: "Оставаться в системе" }));
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json({ status: "verification_required" }, { status: 202 }));
  fireEvent.click(screen.getByRole("button", { name: "Войти" }));
  await screen.findByRole("textbox", { name: "Код из письма: 1" });
  expect(webBrowserFetch).toHaveBeenLastCalledWith("/web/v1/auth/password/login", expect.objectContaining({ body: JSON.stringify({ email: "different@example.test", password: "synthetic-password", persistent: true }) }));
  expect(replaceSignInDocument).not.toHaveBeenCalled();
  fireEvent.paste(screen.getByRole("textbox", { name: "Код из письма: 1" }), { clipboardData: { getData: () => "123456" } });
  fireEvent.click(screen.getByRole("button", { name: "Подтвердить почту" }));
  await vi.waitFor(() => expect(replaceSignInDocument).toHaveBeenCalled());
  expect(webBrowserFetch).toHaveBeenLastCalledWith("/web/v1/auth/password/verify-login", expect.objectContaining({ body: JSON.stringify({ code: "123456" }) }));
});

it("keeps password navigation ordinary even with a remembered email receipt", async () => {
  respond({ email_hint: "m***@example.test" });
  render(<LoginForm methods={methods} entry="choice" />);
  await act(async () => {});
  fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "different@example.test" } });
  fireEvent.click(screen.getByRole("button", { name: "Продолжить" }));
  fireEvent.change(screen.getByLabelText("Пароль"), { target: { value: "unsubmitted-secret" } });
  fireEvent.click(screen.getByRole("button", { name: "Другие способы входа" }));
  expect(screen.getByLabelText("Электронная почта")).toHaveValue("different@example.test");
  expect(screen.queryByRole("button", { name: /Продолжить как/ })).not.toBeInTheDocument();
  fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "different@example.test" } });
  fireEvent.click(screen.getByRole("button", { name: "Продолжить" }));
  expect(screen.getByLabelText("Электронная почта")).toHaveValue("different@example.test");
  expect(screen.getByLabelText("Пароль")).toHaveValue("");
  expect(mutations()).toHaveLength(0);
});

it("forgets only on server success, retains typed email, and offers a safe retry on failure", async () => {
  render(<LoginForm methods={methods} entry="choice" />);
  await screen.findByRole("button", { name: /Продолжить как/ });
  fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "different@example.test" } });
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null, { status: 503 }));
  fireEvent.click(screen.getByRole("button", { name: "Выбрать аккаунт" }));
  fireEvent.click(screen.getByRole("menuitem", { name: "Использовать другой аккаунт" }));
  await screen.findByRole("alert");
  expect(screen.getByRole("button", { name: /Продолжить как/ })).toBeEnabled();
  fireEvent.click(screen.getByRole("button", { name: "Выбрать аккаунт" }));
  fireEvent.click(screen.getByRole("menuitem", { name: "Использовать другой аккаунт" }));
  await vi.waitFor(() => expect(screen.queryByRole("button", { name: /Продолжить как/ })).not.toBeInTheDocument());
  expect(screen.getByRole("button", { name: "Войти с Яндекс ID" })).toBeEnabled();
  expect(screen.getByRole("button", { name: "Войти с VK ID" })).toBeEnabled();
  expect(screen.getByLabelText("Электронная почта")).toHaveValue("different@example.test");
  expect(mutations().map(([path]) => path)).toEqual(["/web/v1/auth/forget", "/web/v1/auth/forget"]);
});

it.each([null, [], { provider: "google", email_hint: "Hidden" }, { provider: "unknown", email_hint: "Unknown" }, { provider: "yandex", email_hint: "x".repeat(129) }])("ignores an invalid or unavailable remembered hint: %j", async value => {
  respond(value);
  render(<LoginForm methods={{ ...methods, providers: ["vk", "google"] }} entry="choice" />);
  await act(async () => {});
  expect(screen.queryByRole("button", { name: /Продолжить как/ })).not.toBeInTheDocument();
  expect(screen.getByLabelText("Электронная почта")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Войти с VK ID" })).toBeEnabled();
  expect(mutations()).toHaveLength(0);
});

it("removes a provider hint when the backend stops offering that provider", async () => {
  const { rerender } = render(<LoginForm methods={methods} entry="choice" />);
  await screen.findByRole("button", { name: /Продолжить как/ });
  rerender(<LoginForm methods={{ ...methods, providers: ["vk"] }} entry="choice" />);
  expect(screen.queryByRole("button", { name: /Продолжить как/ })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Войти с VK ID" })).toBeInTheDocument();
});

it("closes the provider account menu on Escape and outside press without starting authentication", async () => {
  render(<LoginForm methods={methods} entry="choice" />);
  const trigger = await screen.findByRole("button", { name: "Выбрать аккаунт" });
  fireEvent.click(trigger);
  expect(screen.getByRole("menu")).toBeInTheDocument();
  fireEvent.keyDown(document, { key: "Escape" });
  expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  expect(trigger).toHaveFocus();
  fireEvent.click(trigger);
  fireEvent.mouseDown(screen.getByLabelText("Электронная почта"));
  expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  expect(mutations()).toHaveLength(0);
});

it.each(["yandex", "vk"] as const)("demonstrates only the %s provider row and menu without account calls", provider => {
  render(<LoginForm methods={methods} entry="choice" preview previewAccount={provider} />);
  expect(screen.getByRole("button", { name: /Продолжить как Демо-пользователь/ })).toBeDisabled();
  expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Использовать другой аккаунт" })).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Выбрать аккаунт" }));
  fireEvent.click(screen.getByRole("menuitem", { name: "Использовать другой аккаунт" }));
  expect(screen.getByRole("button", { name: "Войти с Яндекс ID" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Войти с VK ID" })).toBeDisabled();
  expect(screen.getByLabelText("Электронная почта")).toHaveValue("");
  expect(webBrowserFetch).not.toHaveBeenCalled();
});

it("ignores a provider demo seed during real sign-in", () => {
  render(<LoginForm methods={methods} entry="email" previewAccount="yandex" />);
  expect(screen.queryByText("Демо-пользователь")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Выбрать аккаунт" })).not.toBeInTheDocument();
  expect(screen.getByLabelText("Электронная почта")).toHaveValue("");
});
