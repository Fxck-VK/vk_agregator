import type { Metadata } from "next";
import { Geist } from "next/font/google";
import { headers } from "next/headers";
import type { ReactNode } from "react";
import { brandIcons } from "@/config/brand";

import { ThemeBootstrapScript } from "@/features/theme/ThemeBootstrapScript";
import { RouteLocaleProvider } from "@/i18n/RouteLocaleProvider";
import { getLocaleDirection } from "@/i18n/locales";
import { getRequestDictionary, getRequestLocale } from "@/i18n/server";

import "./globals.css";

const geistSans = Geist({
  display: "swap",
  subsets: ["cyrillic", "latin", "latin-ext"],
  variable: "--font-geist-sans",
});

const baseMetadata: Metadata = {
  icons: brandIcons,
};

export async function generateMetadata(): Promise<Metadata> {
  const t = await getRequestDictionary();
  return { ...baseMetadata, title: t.document.title, description: t.document.description };
}

export default async function RootLayout({ children }: Readonly<{ children: ReactNode }>) {
  const nonce = (await headers()).get("x-nonce") ?? undefined;
  const locale = await getRequestLocale();

  return (
    <html className={geistSans.variable} data-theme="system" dir={getLocaleDirection(locale)} lang={locale} suppressHydrationWarning>
      <head>
        <ThemeBootstrapScript nonce={nonce} />
      </head>
      <body><RouteLocaleProvider locale={locale}>{children}</RouteLocaleProvider></body>
    </html>
  );
}
