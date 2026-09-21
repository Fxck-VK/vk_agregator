import type { Metadata } from "next";
import { Geist } from "next/font/google";
import { headers } from "next/headers";
import type { ReactNode } from "react";

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
  icons: {
    icon: [
      {
        url: "/assets/brand/favicons/neirohub-favicon-32.png",
        sizes: "32x32",
        type: "image/png",
      },
      {
        url: "/assets/brand/favicons/neirohub-favicon-48.png",
        sizes: "48x48",
        type: "image/png",
      },
    ],
    shortcut: "/assets/brand/favicons/neirohub-favicon-32.png",
    apple: [
      {
        url: "/assets/brand/favicons/neirohub-apple-touch-icon-180.png",
        sizes: "180x180",
        type: "image/png",
      },
    ],
  },
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
