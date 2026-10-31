import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { LocaleProvider } from "@/i18n/LocaleProvider";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { LegalConsentPreferences } from "./LegalConsentPreferences";
import { LegalNotice } from "./LegalLinks";
vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn(), webBrowserMutation: vi.fn() }));
const meta = { published: true, version: "test-version", checksum: "a".repeat(64), marketing_version: "marketing-test", marketing_checksum: "b".repeat(64) };
const status = { ...meta, accepted: false, marketing_opt_in: false, marketing_available: true, recipient_version: "d".repeat(64) };
afterEach(() => { cleanup(); vi.resetAllMocks(); });
it("keeps localized documents linked in the registration notice", () => {
  render(<LocaleProvider locale="en"><LegalNotice /></LocaleProvider>);
  expect(screen.getByRole("link", { name: "User agreement" })).toHaveAttribute("href", "/ru/legal/user-agreement");
});
it("keeps advertising an unchecked independent profile choice", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(status));
  vi.mocked(webBrowserMutation).mockResolvedValue(Response.json({ ...status, marketing_opt_in: true }));
  render(<LegalConsentPreferences metadata={meta} accountId="account-a" />);
  const checkbox = await screen.findByRole("checkbox"); expect(checkbox).not.toBeChecked();
  fireEvent.click(checkbox); fireEvent.click(screen.getByRole("button", { name: "Сохранить выбор" }));
  await screen.findByRole("status");
  expect(vi.mocked(webBrowserMutation).mock.calls[0][0]).toBe("/web/v1/account/legal/marketing");
  expect(JSON.parse(String(vi.mocked(webBrowserMutation).mock.calls[0][1].body))).toMatchObject({ opt_in: true, recipient_version: status.recipient_version });
});
it("allows withdrawal while publication is paused and without current acceptance", async () => {
  vi.mocked(webBrowserMutation).mockResolvedValue(Response.json({ ...status, published: false }));
  render(<LegalConsentPreferences metadata={{ ...meta, published: false }} accountId="account-a" />);
  fireEvent.click(screen.getByRole("button", { name: "Отозвать согласие на рассылку" }));
  await screen.findByRole("status");
  expect(JSON.parse(String(vi.mocked(webBrowserMutation).mock.calls[0][1].body))).toMatchObject({ opt_in: false });
});
