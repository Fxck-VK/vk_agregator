import type { Metadata } from "next";
import Script from "next/script";
import type { ReactNode } from "react";

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

export default function RootLayout({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <html data-theme="system" lang="ru" suppressHydrationWarning>
      <head>
        <Script src="/theme-bootstrap.js" strategy="beforeInteractive" />
      </head>
      <body>{children}</body>
    </html>
  );
}
