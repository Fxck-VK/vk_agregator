import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));
import { webBrowserFetch } from "@/lib/web-api/browser";
import { RecoveryFlow } from "./RecoveryFlow";

const resetBinding = "a".repeat(64);
const otherResetBinding = "b".repeat(64);

beforeEach(() => {
  window.history.replaceState({}, "", "/ru/login/reset");
  vi.mocked(webBrowserFetch).mockImplementation(async path => path.endsWith("/reset-status") || path.endsWith("/prepare-reset")
    ? Response.json({ binding: resetBinding }) : new Response(null, { status: 204 }));
});
afterEach(() => { cleanup(); vi.clearAllMocks(); vi.useRealTimers(); window.history.replaceState({}, "", "/"); });

it("requests a generic reset link, offers delivery help and limits resend", async () => {
  vi.useFakeTimers();
  render(<RecoveryFlow mode="request" preview={false} onBack={vi.fn()} />);
  fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "preview-recovery@example.test" } });
  fireEvent.click(screen.getByRole("button", { name: "Отправить ссылку" }));
  await act(async () => {});
  expect(screen.getByRole("heading", { name: "Проверьте почту" })).toBeInTheDocument();
  expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/auth/password/request-link", expect.objectContaining({ body: JSON.stringify({ email: "preview-recovery@example.test", locale: "ru" }) }));
  expect(screen.getByRole("link", { name: "Помощь с получением письма" })).toHaveAttribute("href", "/ru/ai/docs/receiving-emails");
  expect(screen.getByRole("link", { name: "Написать в поддержку" })).toHaveAttribute("href", "mailto:support@epifera.com");
  expect(screen.getByRole("button", { name: /Отправить ссылку ещё раз/ })).toBeDisabled();
  for (let second = 0; second < 60; second++) act(() => { vi.advanceTimersByTime(1000); });
  fireEvent.click(screen.getByRole("button", { name: "Отправить ссылку ещё раз" }));
  await act(async () => {});
  expect(webBrowserFetch).toHaveBeenCalledTimes(2);
  fireEvent.click(screen.getByRole("button", { name: "Изменить почту" }));
  expect(screen.getByLabelText("Электронная почта")).toHaveValue("preview-recovery@example.test");
});

it("scrubs the reset fragment before preparing the HttpOnly receipt, then validates it", async () => {
  const token = "fictitious-reset-token-for-a-mocked-test";
  window.history.replaceState({ route: "reset", epiferaResetBinding: otherResetBinding }, "", `/ru/login/reset#token=${token}`);
  vi.mocked(webBrowserFetch).mockImplementation(async (path) => {
    expect(window.location.hash).toBe("");
    if (path.endsWith("/prepare-reset")) expect(window.history.state).toEqual({ route: "reset" });
    return Response.json({ binding: resetBinding });
  });
  render(<RecoveryFlow mode="reset" preview={false} onBack={vi.fn()} />);
  await screen.findByLabelText("Новый пароль");
  expect(webBrowserFetch).toHaveBeenNthCalledWith(1, "/web/v1/auth/password/prepare-reset", expect.objectContaining({ body: JSON.stringify({ token }) }));
  expect(webBrowserFetch).toHaveBeenNthCalledWith(2, "/web/v1/auth/password/reset-status", expect.objectContaining({ cache: "no-store", headers: { "X-NeiroHub-Reset-Binding": resetBinding } }));
  expect(window.history.state).toEqual({ route: "reset", epiferaResetBinding: resetBinding });
  expect(JSON.stringify(window.history.state)).not.toContain(token);
  expect(window.location.search).toBe("");
});

it("resumes a prepared reset after reload without requiring the token in the URL", async () => {
  window.history.replaceState({ route: "reset", epiferaResetBinding: resetBinding }, "", "/ru/login/reset");
  render(<RecoveryFlow mode="reset" preview={false} onBack={vi.fn()} />);
  await screen.findByLabelText("Новый пароль");
  expect(webBrowserFetch).toHaveBeenCalledExactlyOnceWith("/web/v1/auth/password/reset-status", expect.objectContaining({ cache: "no-store", headers: { "X-NeiroHub-Reset-Binding": resetBinding } }));
});

it("does not reuse an old receipt when the incoming fragment is malformed", async () => {
  window.history.replaceState({}, "", "/ru/login/reset#token=");
  render(<RecoveryFlow mode="reset" preview={false} onBack={vi.fn()} />);
  await screen.findByRole("heading", { name: "Ссылка недействительна" });
  expect(window.location.hash).toBe("");
  expect(webBrowserFetch).not.toHaveBeenCalled();
});

it("shows expired links with a path to request a replacement", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValue(new Response(null, { status: 410 }));
  render(<RecoveryFlow mode="reset" preview={false} onBack={vi.fn()} />);
  await screen.findByRole("heading", { name: "Ссылка недействительна" });
  expect(screen.getByRole("link", { name: "Запросить новую ссылку" })).toHaveAttribute("href", "/ru/login/recover");
  expect(screen.queryByLabelText("Новый пароль")).not.toBeInTheDocument();
});

it("lets receipt validation retry after a temporary service failure", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null, { status: 503 })).mockResolvedValueOnce(Response.json({ binding: resetBinding }));
  render(<RecoveryFlow mode="reset" preview={false} onBack={vi.fn()} />);
  fireEvent.click(await screen.findByRole("button", { name: "Повторить" }));
  await screen.findByLabelText("Новый пароль");
  expect(webBrowserFetch).toHaveBeenCalledTimes(2);
});

it("blocks mismatched passwords and sends only the matched password after a valid receipt", async () => {
  const onBack = vi.fn();
  render(<RecoveryFlow mode="reset" preview={false} onBack={onBack} />);
  fireEvent.change(await screen.findByLabelText("Новый пароль"), { target: { value: "Fictitious#42Password" } });
  fireEvent.change(screen.getByLabelText("Повторите пароль"), { target: { value: "Different#42" } });
  expect(screen.getByText("Пароли не совпадают.")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Сохранить пароль" })).toBeDisabled();
  expect(webBrowserFetch).toHaveBeenCalledTimes(1);
  fireEvent.change(screen.getByLabelText("Повторите пароль"), { target: { value: "Fictitious#42Password" } });
  fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" }));
  await screen.findByRole("heading", { name: "Пароль обновлён" });
  expect(webBrowserFetch).toHaveBeenLastCalledWith("/web/v1/auth/password/reset-link", expect.objectContaining({ body: JSON.stringify({ new_password: "Fictitious#42Password" }), headers: { "Content-Type": "application/json", "X-NeiroHub-Reset-Binding": resetBinding } }));
  expect(window.history.state.epiferaResetBinding).toBeUndefined();
  fireEvent.click(screen.getByRole("button", { name: "Вернуться ко входу" }));
  expect(onBack).toHaveBeenCalledOnce();
});

it("keeps preview request and reset entirely free of API writes or reads", async () => {
  const first = render(<RecoveryFlow mode="request" preview onBack={vi.fn()} />);
  fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "preview-recovery@example.test" } });
  fireEvent.click(screen.getByRole("button", { name: "Отправить ссылку" }));
  await screen.findByRole("heading", { name: "Проверьте почту" });
  first.unmount();
  render(<RecoveryFlow mode="reset" preview onBack={vi.fn()} />);
  fireEvent.change(screen.getByLabelText("Новый пароль"), { target: { value: "Fictitious#42Password" } });
  fireEvent.change(screen.getByLabelText("Повторите пароль"), { target: { value: "Fictitious#42Password" } });
  fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" }));
  await screen.findByRole("heading", { name: "Пароль обновлён" });
  expect(webBrowserFetch).not.toHaveBeenCalled();
});

it("honors a server resend delay and cannot bypass it with another form submit", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null, { status: 429, headers: { "Retry-After": "120" } }));
  const { container } = render(<RecoveryFlow mode="request" preview={false} onBack={vi.fn()} />);
  fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "preview-recovery@example.test" } });
  fireEvent.submit(container.querySelector("form")!);
  await screen.findByText("Слишком много попыток. Подождите и попробуйте позже.");
  expect(screen.getByRole("button", { name: "Отправить ссылку (120)" })).toBeDisabled();
  fireEvent.submit(container.querySelector("form")!);
  expect(webBrowserFetch).toHaveBeenCalledTimes(1);
});

it("keeps the same password for an ambiguous reset retry", async () => {
  vi.mocked(webBrowserFetch)
    .mockResolvedValueOnce(Response.json({ binding: resetBinding }))
    .mockResolvedValueOnce(new Response(null, { status: 503 }))
    .mockResolvedValueOnce(new Response(null, { status: 204 }));
  render(<RecoveryFlow mode="reset" preview={false} onBack={vi.fn()} />);
  fireEvent.change(await screen.findByLabelText("Новый пароль"), { target: { value: "Fictitious#42Password" } });
  fireEvent.change(screen.getByLabelText("Повторите пароль"), { target: { value: "Fictitious#42Password" } });
  fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" }));
  await screen.findByText("Не удалось подтвердить смену пароля. Повторите с тем же паролем или попробуйте войти.");
  expect(screen.getByLabelText("Новый пароль")).toHaveAttribute("readonly");
  fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" }));
  await screen.findByRole("heading", { name: "Пароль обновлён" });
  expect(vi.mocked(webBrowserFetch).mock.calls.filter(([path]) => path.endsWith("/reset-link")).map(([, options]) => options?.body)).toEqual([JSON.stringify({ new_password: "Fictitious#42Password" }), JSON.stringify({ new_password: "Fictitious#42Password" })]);
});

it("adopts a prepared cookie only when the tab has no previous reset binding", async () => {
  window.history.replaceState({ preserved: "next-route-state" }, "", "/ru/login/reset");
  render(<RecoveryFlow mode="reset" preview={false} onBack={vi.fn()} />);
  await screen.findByLabelText("Новый пароль");
  expect(window.history.state).toEqual({ preserved: "next-route-state", epiferaResetBinding: resetBinding });
});

it("rejects a different reset cookie after reload instead of adopting the other account", async () => {
  window.history.replaceState({ preserved: true, epiferaResetBinding: resetBinding }, "", "/ru/login/reset");
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ binding: otherResetBinding }));
  render(<RecoveryFlow mode="reset" preview={false} onBack={vi.fn()} />);
  await screen.findByRole("heading", { name: "Ссылка недействительна" });
  expect(webBrowserFetch).toHaveBeenCalledExactlyOnceWith("/web/v1/auth/password/reset-status", expect.objectContaining({ headers: { "X-NeiroHub-Reset-Binding": resetBinding } }));
  expect(screen.queryByLabelText("Новый пароль")).not.toBeInTheDocument();
  expect(window.history.state).toEqual({ preserved: true });
});

it("keeps the tab binding when another tab replaces the shared reset cookie", async () => {
  let cookieBinding = resetBinding;
  let resets = 0;
  vi.mocked(webBrowserFetch).mockImplementation(async (path, options) => {
    if (path.endsWith("/reset-status")) return Response.json({ binding: cookieBinding });
    if (path.endsWith("/reset-link")) {
      const requested = new Headers(options?.headers).get("X-NeiroHub-Reset-Binding");
      expect(requested).toBe(resetBinding);
      if (requested !== cookieBinding) return new Response(null, { status: 400 });
      resets++;
      return new Response(null, { status: 204 });
    }
    throw new Error("Unexpected test request");
  });
  render(<RecoveryFlow mode="reset" preview={false} onBack={vi.fn()} />);
  fireEvent.change(await screen.findByLabelText("Новый пароль"), { target: { value: "Fictitious#42Password" } });
  fireEvent.change(screen.getByLabelText("Повторите пароль"), { target: { value: "Fictitious#42Password" } });
  cookieBinding = otherResetBinding;
  fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" }));
  await screen.findByRole("heading", { name: "Ссылка недействительна" });
  expect(resets).toBe(0);
  expect(webBrowserFetch).toHaveBeenCalledTimes(2);
});

it("refuses malformed public binding metadata and never enables reset submission", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ binding: "not-a-digest" }));
  render(<RecoveryFlow mode="reset" preview={false} onBack={vi.fn()} />);
  await screen.findByRole("button", { name: "Повторить" });
  expect(screen.queryByLabelText("Новый пароль")).not.toBeInTheDocument();
  expect(window.history.state.epiferaResetBinding).toBeUndefined();
});
