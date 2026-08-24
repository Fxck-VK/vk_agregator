import type { Metadata } from "next";
import { headers } from "next/headers";
import type { ReactNode } from "react";

import { themeBootstrapScript } from "@/features/theme/theme-preference";
import { ru } from "@/i18n/ru";

import "./globals.css";

export const metadata: Metadata = {
  title: ru.document.title,
  description: ru.document.description,
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

export default async function RootLayout({ children }: Readonly<{ children: ReactNode }>) {
  const nonce = (await headers()).get("x-nonce") ?? undefined;

  return (
    <html data-theme="system" lang="ru" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeBootstrapScript }} nonce={nonce} />
      </head>
      <body>{children}</body>
    </html>
  );
}
