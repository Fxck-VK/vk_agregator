"use server";

import { cookies } from "next/headers";
import { localeCookieName, parseSiteLocale } from "./locales";

export async function setLocalePreference(input: string) {
  const locale = parseSiteLocale(input);
  (await cookies()).set(localeCookieName, locale, {
    path: "/",
    sameSite: "lax",
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    maxAge: 60 * 60 * 24 * 365,
  });
}
