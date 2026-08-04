import type { Metadata } from "next";
import { headers } from "next/headers";
import type { ReactNode } from "react";

import { themeBootstrapScript } from "@/features/theme/theme-preference";
import { publicSiteOrigin } from "@/features/landing/seo/landing-json-ld";

import "./globals.css";

export const metadata: Metadata = {
  metadataBase: new URL(publicSiteOrigin),
  title: {
    default: "NeiroHub — нейросети на русском в одном месте",
    template: "%s | NeiroHub",
  },
  description: "Единая веб-платформа для общения с нейросетями, создания контента и хранения результатов.",
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
