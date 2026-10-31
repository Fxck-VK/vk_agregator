import { beforeEach, expect, it, vi } from "vitest";
vi.mock("server-only", () => ({}));
vi.mock("next/headers", () => ({ cookies: vi.fn(), headers: vi.fn() }));
import { cookies, headers } from "next/headers";
import { getRequestDictionary, getRequestLocale } from "./server";
import { setLocalePreference } from "./actions";
import { localeCookieName } from "./locales";
import { localeRequestHeader } from "./routing";

const set = vi.fn();
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(cookies).mockResolvedValue({ get: () => undefined, set } as never);
  vi.mocked(headers).mockResolvedValue(new Headers() as never);
});
it("resolves even legacy locale headers to Russian without request-global mutation", async () => {
  vi.mocked(headers).mockResolvedValueOnce(new Headers({ [localeRequestHeader]: "en" }) as never)
    .mockResolvedValueOnce(new Headers({ [localeRequestHeader]: "ru" }) as never);
  const [english, russian] = await Promise.all([getRequestDictionary(), getRequestDictionary()]);
  expect(english.navigation.files).toBe("Мои файлы");
  expect(russian.navigation.files).toBe("Мои файлы");
  expect(await getRequestLocale()).toBe("ru");
});
it("never lets the preference cookie determine page language", async () => {
  vi.mocked(cookies).mockResolvedValue({ get: () => ({ value: "en" }) } as never);
  expect(await getRequestLocale()).toBe("ru");
});
it("validates and persists a shared preference without reading or mutating account data", async () => {
  await setLocalePreference("ru");
  expect(set).toHaveBeenCalledWith(localeCookieName, "ru", expect.objectContaining({ path: "/", httpOnly: true, sameSite: "lax", maxAge: 31536000 }));
  await expect(setLocalePreference("en")).rejects.toThrow("Unsupported locale for website");
  await expect(setLocalePreference("unknown")).rejects.toThrow("Unsupported locale");
  expect(set).toHaveBeenCalledTimes(1);
});
