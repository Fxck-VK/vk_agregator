import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AccountSecurity } from "./AccountSecurity";
import { previewAuthMethods } from "@/lib/auth/methods";
import { localWorkspacePreviewProfile } from "@/features/session/local-workspace-preview";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { requestWorkspaceLogout } from "@/features/session/WorkspaceLogout/workspace-logout-request";

const navigation = vi.hoisted(() => ({ replace: vi.fn(), refresh: vi.fn() }));
vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn(), webBrowserMutation: vi.fn() }));
vi.mock("@/features/session/WorkspaceLogout/workspace-logout-request", () => ({ requestWorkspaceLogout: vi.fn() }));
vi.mock("server-only", () => ({}));
vi.mock("next/navigation", () => ({ useRouter: () => navigation }));

afterEach(() => { cleanup(); vi.clearAllMocks(); });
describe("AccountSecurity", () => {
  it.each(["apple", "telegram", "vk"])("protects email when the remaining %s binding has no browser login", (provider) => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ items: [] }));
    const profile = { ...localWorkspacePreviewProfile, password_set: true, identity_refs: [
      { ...localWorkspacePreviewProfile.identity_refs[0], provider: "email" },
      { ...localWorkspacePreviewProfile.identity_refs[0], id: "10000000-0000-4000-8000-000000000006", provider, label: "Stored binding" },
    ] };
    render(<AccountSecurity profile={profile} methods={{ ...previewAuthMethods, providers: [] }} />);
    const emailRow = screen.getByText("Электронная почта").closest("li")!;
    expect(within(emailRow).getByRole("button", { name: "Отвязать" })).toBeDisabled();
  });
  const twoEmailProfile = () => ({ ...localWorkspacePreviewProfile, identity_refs: [
    { ...localWorkspacePreviewProfile.identity_refs[0], email_role: "primary" as const },
    { ...localWorkspacePreviewProfile.identity_refs[0], id: "10000000-0000-4000-8000-000000000003", label: "b***@example.test", email_role: "backup" as const },
  ] });
  it("labels both roles and replaces the backup only after confirming the new address", async () => {
    const profile = twoEmailProfile(); const backup = { ...profile.identity_refs[1], label: "n***@example.test" };
    vi.mocked(webBrowserFetch).mockImplementation(async path => path === "/web/v1/me" ? Response.json({ ...profile, identity_refs: [profile.identity_refs[0], backup] }) : Response.json({ items: [] }));
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 202 }));
    render(<AccountSecurity profile={profile} methods={previewAuthMethods} />);
    expect(screen.getByText("Основная почта")).toBeInTheDocument(); expect(screen.getByText("Резервная почта")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Заменить" }));
    expect(screen.getByText(/До подтверждения текущая резервная почта продолжит работать/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "new@example.test" } });
    fireEvent.click(screen.getByRole("button", { name: "Отправить код" })); await screen.findByLabelText("Код из письма");
    expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/account/identities/email/backup/request-code", expect.objectContaining({ body: JSON.stringify({ email: "new@example.test", identity_id: backup.id }) }));
    expect(screen.getByText("b***@example.test")).toBeInTheDocument(); expect(screen.queryByText("n***@example.test")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Код из письма"), { target: { value: "123456" } });
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json(backup));
    fireEvent.click(screen.getByRole("button", { name: "Подтвердить" }));
    await screen.findByText("Резервная почта заменена."); expect(await screen.findByText("n***@example.test")).toBeInTheDocument();
    expect(webBrowserMutation).toHaveBeenLastCalledWith("/web/v1/account/identities/email/backup/verify", expect.objectContaining({ body: JSON.stringify({ email: "new@example.test", identity_id: backup.id, code: "123456" }) }));
    expect(screen.queryByRole("button", { name: "Добавить резервную почту" })).not.toBeInTheDocument();
  });
  it("resets consumed proof on conflict and retains the old backup", async () => {
    const profile = twoEmailProfile();
    vi.mocked(webBrowserFetch).mockImplementation(async path => path === "/web/v1/me" ? Response.json(profile) : Response.json({ items: [] }));
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 202 }));
    render(<AccountSecurity profile={profile} methods={previewAuthMethods} />);
    fireEvent.click(screen.getByRole("button", { name: "Заменить" }));
    fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "occupied@example.test" } });
    fireEvent.click(screen.getByRole("button", { name: "Отправить код" })); await screen.findByLabelText("Код из письма");
    fireEvent.change(screen.getByLabelText("Код из письма"), { target: { value: "123456" } });
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json({ error: "email verification failed" }, { status: 409 }));
    fireEvent.click(screen.getByRole("button", { name: "Подтвердить" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Этот способ входа уже связан с другим аккаунтом.");
    await waitFor(() => expect(screen.getByLabelText("Электронная почта")).toBeEnabled());
    expect(screen.queryByLabelText("Код из письма")).not.toBeInTheDocument(); expect(screen.getByText("b***@example.test")).toBeInTheDocument();
  });
  it("refreshes changed backup state instead of retrying stale proof", async () => {
    const profile = twoEmailProfile();
    vi.mocked(webBrowserFetch).mockImplementation(async path => path === "/web/v1/me" ? Response.json(profile) : Response.json({ items: [] }));
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json({ error: "backup_email_changed" }, { status: 409 }));
    render(<AccountSecurity profile={profile} methods={previewAuthMethods} />);
    fireEvent.click(screen.getByRole("button", { name: "Заменить" }));
    fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "new@example.test" } });
    fireEvent.click(screen.getByRole("button", { name: "Отправить код" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Резервная почта уже изменилась.");
    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/me"); expect(screen.queryByLabelText("Электронная почта")).not.toBeInTheDocument();
  });
  it("hides adding another email when primary and backup are present", () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ items: [] }));
    const profile = { ...localWorkspacePreviewProfile, identity_refs: [localWorkspacePreviewProfile.identity_refs[0], { ...localWorkspacePreviewProfile.identity_refs[0], id: "10000000-0000-4000-8000-000000000003", label: "b***@example.test" }] };
    render(<AccountSecurity profile={profile} methods={previewAuthMethods} preview />);
    expect(screen.queryByRole("button", { name: "Добавить резервную почту" })).not.toBeInTheDocument();
  });
  it("confirms a backup email by code before showing it as linked", async () => {
    const profile = { ...localWorkspacePreviewProfile, identity_refs: [...localWorkspacePreviewProfile.identity_refs, { ...localWorkspacePreviewProfile.identity_refs[0], id: "10000000-0000-4000-8000-000000000003", label: "b***@example.test" }] };
    vi.mocked(webBrowserFetch).mockImplementation(async path => path === "/web/v1/me" ? Response.json(profile) : Response.json({ items: [] }));
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 202 }));
    render(<AccountSecurity profile={localWorkspacePreviewProfile} methods={previewAuthMethods} />);
    fireEvent.click(screen.getByRole("button", { name: "Добавить резервную почту" }));
    expect(screen.getByText("Подтвердите дополнительный адрес. Он позволит войти и восстановить доступ, если основная почта недоступна.")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "backup@example.test" } });
    fireEvent.click(screen.getByRole("button", { name: "Отправить код" }));
    await screen.findByLabelText("Код из письма");
    expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/account/identities/email/request-code", expect.objectContaining({ body: JSON.stringify({ email: "backup@example.test" }) }));
    expect(screen.queryByText("b***@example.test")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Электронная почта")).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Код из письма"), { target: { value: "123456" } });
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json(profile.identity_refs[1], { status: 201 }));
    fireEvent.click(screen.getByRole("button", { name: "Подтвердить" }));
    await screen.findByText("Резервная почта подтверждена. Для восстановления доступа укажите её на странице входа.");
    expect(webBrowserMutation).toHaveBeenLastCalledWith("/web/v1/account/identities/email/verify", expect.objectContaining({ body: JSON.stringify({ email: "backup@example.test", code: "123456" }) }));
    expect(await screen.findByText("b***@example.test")).toBeInTheDocument();
    expect(screen.queryByLabelText("Код из письма")).not.toBeInTheDocument();
  });
  it("keeps the backup unlinked when the verification code is rejected", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ items: [] }));
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 202 }));
    render(<AccountSecurity profile={localWorkspacePreviewProfile} methods={previewAuthMethods} />);
    fireEvent.click(screen.getByRole("button", { name: "Добавить резервную почту" }));
    fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "backup@example.test" } });
    fireEvent.click(screen.getByRole("button", { name: "Отправить код" }));
    await screen.findByLabelText("Код из письма");
    fireEvent.change(screen.getByLabelText("Код из письма"), { target: { value: "000000" } });
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json({ error: "email verification failed" }, { status: 400 }));
    fireEvent.click(screen.getByRole("button", { name: "Подтвердить" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Не удалось выполнить действие.");
    expect(screen.getByLabelText("Код из письма")).toBeInTheDocument();
    expect(webBrowserFetch).not.toHaveBeenCalledWith("/web/v1/me");
    expect(screen.queryByText(/Резервная почта подтверждена/)).not.toBeInTheDocument();
  });
  it("asks for another address instead of claiming the existing email is a new backup", async () => {
    vi.mocked(webBrowserFetch).mockImplementation(async path => path === "/web/v1/me" ? Response.json(localWorkspacePreviewProfile) : Response.json({ items: [] }));
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 202 }));
    render(<AccountSecurity profile={localWorkspacePreviewProfile} methods={previewAuthMethods} />);
    fireEvent.click(screen.getByRole("button", { name: "Добавить резервную почту" }));
    fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "preview@neirohub.local" } });
    fireEvent.click(screen.getByRole("button", { name: "Отправить код" }));
    await screen.findByLabelText("Код из письма");
    fireEvent.change(screen.getByLabelText("Код из письма"), { target: { value: "123456" } });
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json(localWorkspacePreviewProfile.identity_refs[0], { status: 201 }));
    fireEvent.click(screen.getByRole("button", { name: "Подтвердить" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Эта почта уже добавлена. Укажите другой адрес.");
    expect(screen.getByLabelText("Электронная почта")).toBeEnabled();
    expect(screen.queryByText(/Резервная почта подтверждена/)).not.toBeInTheDocument();
  });
  it("offers adding the first email when no verified email is linked", () => {
    render(<AccountSecurity profile={{ ...localWorkspacePreviewProfile, identity_refs: localWorkspacePreviewProfile.identity_refs.map(identity => ({ ...identity, provider: "google" })) }} methods={previewAuthMethods} preview />);
    expect(screen.getByRole("button", { name: "Добавить почту" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Добавить резервную почту" })).not.toBeInTheDocument();
  });
  it("shows linked Google as disabled, hides stale Google linking, and keeps the primary email protected", () => {
    const profile = {
      ...localWorkspacePreviewProfile,
      identity_refs: [
        { ...localWorkspacePreviewProfile.identity_refs[0], email_role: "primary" as const },
        {
          ...localWorkspacePreviewProfile.identity_refs[0],
          id: "10000000-0000-4000-8000-000000000003",
          provider: "google",
          label: "google-member@example.test",
        },
      ],
    };

    render(<AccountSecurity profile={profile} methods={{ ...previewAuthMethods, providers: ["google"] }} />);

    expect(screen.getByText("Отключено")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Привязать Google" })).not.toBeInTheDocument();
    expect(screen.getByText("Сохраните хотя бы один способ входа.")).toBeInTheDocument();

    const emailRow = screen.getByText("Основная почта").closest("li");
    const googleRow = screen.getByText("Google").closest("li");

    expect(emailRow).not.toBeNull();
    expect(googleRow).not.toBeNull();
    expect(within(emailRow!).getByRole("button", { name: "Отвязать" })).toBeDisabled();
    expect(within(googleRow!).getByRole("button", { name: "Отвязать" })).toBeEnabled();
  });
  it("does not treat an unconfirmed email as an existing recovery address", () => {
    render(<AccountSecurity profile={{ ...localWorkspacePreviewProfile, identity_refs: localWorkspacePreviewProfile.identity_refs.map(identity => ({ ...identity, verified: false })) }} methods={previewAuthMethods} preview />);
    expect(screen.getByRole("button", { name: "Добавить почту" })).toBeInTheDocument();
  });
  it("requires current password and matching new passwords before changing an existing password", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ items: [] }));
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 204 }));
    render(<AccountSecurity profile={{ ...localWorkspacePreviewProfile, password_set: true }} methods={previewAuthMethods} />);
    fireEvent.click(screen.getByRole("button", { name: "Изменить пароль" }));
    expect(screen.queryByRole("button", { name: "Установить пароль" })).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "member@example.test" } });
    fireEvent.change(screen.getByLabelText("Текущий пароль"), { target: { value: "original-password" } });
    fireEvent.change(screen.getByLabelText("Новый пароль"), { target: { value: "replacement-password" } });
    fireEvent.change(screen.getByLabelText("Повторите пароль"), { target: { value: "different-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" }));
    expect(await screen.findByText("Пароли не совпадают.")).toBeInTheDocument();
    expect(webBrowserMutation).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText("Повторите пароль"), { target: { value: "replacement-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" }));
    await screen.findByText("Пароль сохранён.");
    expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/account/password/set", expect.objectContaining({ body: JSON.stringify({ email: "member@example.test", password: "replacement-password", current_password: "original-password" }) }));
  });
  it("reports wrong current password locally and clears all submitted password fields", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ items: [] }));
    vi.mocked(webBrowserMutation).mockResolvedValue(Response.json({ error: "current_password_invalid" }, { status: 400 }));
    render(<AccountSecurity profile={{ ...localWorkspacePreviewProfile, password_set: true }} methods={previewAuthMethods} />);
    fireEvent.click(screen.getByRole("button", { name: "Изменить пароль" }));
    fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "member@example.test" } });
    fireEvent.change(screen.getByLabelText("Текущий пароль"), { target: { value: "incorrect-password" } });
    fireEvent.change(screen.getByLabelText("Новый пароль"), { target: { value: "replacement-password" } });
    fireEvent.change(screen.getByLabelText("Повторите пароль"), { target: { value: "replacement-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" }));
    await screen.findByText("Текущий пароль неверный. Попробуйте ещё раз.");
    for (const label of ["Текущий пароль", "Новый пароль", "Повторите пароль"]) expect(screen.getByLabelText(label)).toHaveValue("");
    expect(requestWorkspaceLogout).not.toHaveBeenCalled();
  });
  it("offers first password setup only when the server confirms no password exists", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ items: [] }));
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 204 }));
    render(<AccountSecurity profile={{ ...localWorkspacePreviewProfile, password_set: false }} methods={previewAuthMethods} />);
    fireEvent.click(screen.getByRole("button", { name: "Установить пароль" }));
    expect(screen.queryByLabelText("Текущий пароль")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Электронная почта"), { target: { value: "member@example.test" } });
    for (const label of ["Новый пароль", "Повторите пароль"]) fireEvent.change(screen.getByLabelText(label), { target: { value: "initial-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить пароль" }));
    await screen.findByRole("button", { name: "Изменить пароль" });
    expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/account/password/set", expect.objectContaining({ body: JSON.stringify({ email: "member@example.test", password: "initial-password" }) }));
  });
  it("does not guess the password status when profile metadata is absent", () => {
    render(<AccountSecurity profile={localWorkspacePreviewProfile} methods={previewAuthMethods} preview />);
    expect(screen.queryByRole("button", { name: "Установить пароль" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Изменить пароль" })).not.toBeInTheDocument();
  });
  it("ends the live cookie session even when refresh has rotated the listed session ID", async () => {
    const id = "31000000-0000-4000-8000-000000000002";
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ items: [{ id, account_id: localWorkspacePreviewProfile.account_id, created_at: "2026-10-01T12:00:00Z", updated_at: "2026-10-01T12:00:00Z", expires_at: "2026-10-31T12:00:00Z", revoked: false, current: true }] }));
    vi.mocked(requestWorkspaceLogout).mockResolvedValue();
    render(<AccountSecurity profile={localWorkspacePreviewProfile} methods={previewAuthMethods} />);
    await screen.findByText("Текущая сессия");
    fireEvent.click(screen.getByRole("button", { name: "Завершить" }));
    fireEvent.click(screen.getAllByRole("button", { name: "Завершить" }).at(-1)!);
    await waitFor(() => expect(requestWorkspaceLogout).toHaveBeenCalledOnce());
    expect(webBrowserMutation).not.toHaveBeenCalled();
    expect(navigation.replace).toHaveBeenCalledWith("/ru/login");
  });
  it("shows current sessions and revokes only the selected session through the CSRF mutation helper", async () => {
    const id = "31000000-0000-4000-8000-000000000001";
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ items: [{ id, account_id: localWorkspacePreviewProfile.account_id, created_at: "2026-10-01T12:00:00Z", updated_at: "2026-10-01T12:00:00Z", expires_at: "2026-10-31T12:00:00Z", revoked: false, current: false }] }));
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 204 }));
    render(<AccountSecurity profile={localWorkspacePreviewProfile} methods={previewAuthMethods} />);
    await screen.findByText("Другой вход");
    fireEvent.click(screen.getByRole("button", { name: "Завершить" }));
    expect(webBrowserMutation).not.toHaveBeenCalled();
    fireEvent.click(screen.getAllByRole("button", { name: "Завершить" }).at(-1)!);
    await screen.findByText("Других активных сессий нет.");
    expect(webBrowserMutation).toHaveBeenCalledWith(`/web/v1/account/sessions/${id}/revoke`, expect.objectContaining({ method: "POST" }));
  });
  it("does not offer unlinking the last identity and never performs writes in preview", () => {
    render(<AccountSecurity profile={localWorkspacePreviewProfile} methods={previewAuthMethods} preview />);
    expect(screen.getByRole("button", { name: "Отвязать" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Добавить резервную почту" }));
    expect(webBrowserMutation).not.toHaveBeenCalled();
  });
});
