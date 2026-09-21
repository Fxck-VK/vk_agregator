"use server";

import { cookies } from "next/headers";
import { localeCookieName, parseLocale } from "./locales";

export async function setLocalePreference(input: string) {
  const locale = parseLocale(input);
  (await cookies()).set(localeCookieName, locale, {
    path: "/",
    sameSite: "lax",
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    maxAge: 60 * 60 * 24 * 365,
  });
}
