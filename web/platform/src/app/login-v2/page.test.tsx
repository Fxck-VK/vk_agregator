import { beforeEach, expect, it, vi } from "vitest";
vi.mock("server-only", () => ({}));
vi.mock("next/navigation", () => ({ redirect: vi.fn(() => { throw new Error("redirect"); }) }));
vi.mock("@/i18n/server", async original => ({ ...await original<typeof import("@/i18n/server")>(), getRequestLocale: vi.fn() }));
import { redirect } from "next/navigation";
import { getRequestLocale } from "@/i18n/server";
import Page, { metadata } from "./page";
beforeEach(() => { vi.clearAllMocks(); vi.mocked(getRequestLocale).mockResolvedValue("ru"); });
it("retires the alternate URL through the canonical localized login page", async () => {
  await expect(Page()).rejects.toThrow("redirect"); expect(redirect).toHaveBeenCalledWith("/ru/login");
  expect(metadata.robots).toEqual({ index: false, follow: false });
});
it("maps registration to its route and forwards only the known OAuth error", async () => {
  await expect(Page({ searchParams: Promise.resolve({ mode: "register", oauth: "failed", step: "password", returnTo: "https://untrusted.example/", token: "fictitious-query-token", email: "preview@example.test" }) })).rejects.toThrow("redirect");
  expect(redirect).toHaveBeenCalledWith("/ru/login/register?oauth=failed");
});
it("retains only the allowed password step and request locale", async () => {
  vi.mocked(getRequestLocale).mockResolvedValue("en");
  await expect(Page({ searchParams: Promise.resolve({ step: "password", oauth: "failed", returnTo: "/app/files", unknown: "discard" }) })).rejects.toThrow("redirect");
  expect(redirect).toHaveBeenCalledWith("/ru/login?oauth=failed&step=password");
});
it("ignores unrecognized and multi-valued navigation hints", async () => {
  await expect(Page({ searchParams: Promise.resolve({ mode: ["register"], oauth: "other", step: ["password"], next: "/app" }) })).rejects.toThrow("redirect");
  expect(redirect).toHaveBeenCalledWith("/ru/login");
});
