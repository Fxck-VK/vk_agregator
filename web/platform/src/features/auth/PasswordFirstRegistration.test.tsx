import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));
vi.mock("@/lib/auth/sign-in-navigation", () => ({ replaceSignInDocument: vi.fn() }));
import { webBrowserFetch } from "@/lib/web-api/browser";
import { replaceSignInDocument } from "@/lib/auth/sign-in-navigation";
import { EmailRegistration } from "./EmailRegistration";

// Deliberately fictitious credentials, used only with mocked requests.
const email = "registration@example.test";
const password = "Sunset#42River!";
const legalMetadata = { published: true, version: "v1", checksum: "a".repeat(64), marketing_version: "m1", marketing_checksum: "b".repeat(64) };
beforeEach(() => {
  vi.mocked(webBrowserFetch).mockResolvedValue(new Response(null, { status: 204 }));
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ account_id: "10000000-0000-4000-8000-000000000001", identity_refs: [] })));
});
afterEach(() => { cleanup(); vi.clearAllMocks(); vi.unstubAllGlobals(); });

async function begin(preview = false) {
  const onBack = vi.fn();
  render(<EmailRegistration passwordFirst preview={preview} initialEmail={email} onBack={onBack} legalMetadata={legalMetadata} />);
  expect(screen.getByLabelText("Электронная почта")).toHaveValue(email);
  expect(screen.getByRole("button", { name: "Зарегистрироваться" })).toBeDisabled();
  fireEvent.change(screen.getByLabelText("Пароль"), { target: { value: password } });
  fireEvent.click(screen.getByRole("button", { name: "Зарегистрироваться" }));
  await screen.findByRole("textbox", { name: "Код из письма: 1" });
  return onBack;
}

it("collects email/password before requesting code and registers only after successful server verification", async () => {
  await begin();
  expect(webBrowserFetch).toHaveBeenCalledTimes(1);
  expect(webBrowserFetch).toHaveBeenLastCalledWith("/web/v1/auth/email/request-code", expect.objectContaining({ body: JSON.stringify({ email }) }));
  expect(screen.queryByLabelText("Пароль")).not.toBeInTheDocument();
  fireEvent.change(screen.getByRole("textbox", { name: "Код из письма: 1" }), { target: { value: "123456" } });
  fireEvent.click(screen.getByRole("button", { name: "Подтвердить почту" }));
  await vi.waitFor(() => expect(replaceSignInDocument).toHaveBeenCalledWith("/ru/app"));
  expect(vi.mocked(webBrowserFetch).mock.calls.map(([path]) => path)).toEqual(["/web/v1/auth/email/request-code", "/web/v1/auth/email/verify-code", "/web/v1/auth/email/register"]);
  expect(webBrowserFetch).toHaveBeenLastCalledWith("/web/v1/auth/email/register", expect.objectContaining({ body: JSON.stringify({ email, password, registration: { version: legalMetadata.version, checksum: legalMetadata.checksum, locale: "ru" } }) }));
});

it("does not register on an incorrect code and retains the chosen password for the corrected code", async () => {
  await begin();
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null, { status: 400 }));
  fireEvent.change(screen.getByRole("textbox", { name: "Код из письма: 1" }), { target: { value: "000000" } });
  fireEvent.click(screen.getByRole("button", { name: "Подтвердить почту" }));
  await screen.findByRole("alert");
  expect(vi.mocked(webBrowserFetch).mock.calls.some(([path]) => path.endsWith("/register"))).toBe(false);
  fireEvent.change(screen.getByRole("textbox", { name: "Код из письма: 1" }), { target: { value: "123456" } });
  fireEvent.click(screen.getByRole("button", { name: "Подтвердить почту" }));
  await vi.waitFor(() => expect(replaceSignInDocument).toHaveBeenCalled());
  expect(webBrowserFetch).toHaveBeenLastCalledWith("/web/v1/auth/email/register", expect.objectContaining({ body: expect.stringContaining(password) }));
});

it("retries registration with the original password without consuming the verified code again", async () => {
  await begin();
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null, { status: 204 })).mockResolvedValueOnce(new Response(null, { status: 503 }));
  fireEvent.change(screen.getByRole("textbox", { name: "Код из письма: 1" }), { target: { value: "123456" } });
  fireEvent.click(screen.getByRole("button", { name: "Подтвердить почту" }));
  await screen.findByRole("alert");
  fireEvent.click(screen.getByRole("button", { name: "Создать аккаунт" }));
  await vi.waitFor(() => expect(replaceSignInDocument).toHaveBeenCalled());
  const calls = vi.mocked(webBrowserFetch).mock.calls;
  expect(calls.filter(([path]) => path.endsWith("/verify-code"))).toHaveLength(1);
  const writes = calls.filter(([path]) => path.endsWith("/register"));
  expect(writes).toHaveLength(2);
  expect(writes[0][1]?.body).toBe(writes[1][1]?.body);
});

it("retries session confirmation without creating the account again", async () => {
  await begin();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response(null, { status: 401 })).mockResolvedValueOnce(Response.json({ account_id: "10000000-0000-4000-8000-000000000001", identity_refs: [] })));
  fireEvent.change(screen.getByRole("textbox", { name: "Код из письма: 1" }), { target: { value: "123456" } });
  fireEvent.click(screen.getByRole("button", { name: "Подтвердить почту" }));
  await screen.findByRole("alert");
  fireEvent.click(screen.getByRole("button", { name: "Повторить вход" }));
  await vi.waitFor(() => expect(replaceSignInDocument).toHaveBeenCalled());
  expect(vi.mocked(webBrowserFetch).mock.calls.filter(([path]) => path.endsWith("/register"))).toHaveLength(1);
});

it("clears the previous password when changing email and makes no writes in preview", async () => {
  await begin(true);
  fireEvent.click(screen.getByRole("button", { name: "Изменить почту" }));
  expect(screen.getByLabelText("Пароль")).toHaveValue("");
  expect(screen.getByRole("button", { name: "Зарегистрироваться" })).toBeDisabled();
  expect(webBrowserFetch).not.toHaveBeenCalled();
});

it("returns to an empty password field after final verification expires", async () => {
  await begin();
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null, { status: 204 })).mockResolvedValueOnce(new Response(null, { status: 400 }));
  fireEvent.change(screen.getByRole("textbox", { name: "Код из письма: 1" }), { target: { value: "123456" } });
  fireEvent.click(screen.getByRole("button", { name: "Подтвердить почту" }));
  await screen.findByText("Подтверждение истекло. Запросите новый код.");
  expect(screen.getByLabelText("Пароль")).toHaveValue("");
  expect(screen.getByRole("button", { name: "Зарегистрироваться" })).toBeDisabled();
});

it("completes the preview after code without another password step or any account write", async () => {
  await begin(true);
  fireEvent.change(screen.getByRole("textbox", { name: "Код из письма: 1" }), { target: { value: "123456" } });
  fireEvent.click(screen.getByRole("button", { name: "Подтвердить почту" }));
  await screen.findByText("Предпросмотр завершён. На рабочем сайте здесь создаётся аккаунт и выполняется вход.");
  expect(screen.queryByLabelText("Пароль")).not.toBeInTheDocument();
  expect(webBrowserFetch).not.toHaveBeenCalled();
  expect(replaceSignInDocument).not.toHaveBeenCalled();
});
