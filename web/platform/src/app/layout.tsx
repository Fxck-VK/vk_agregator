import type { Metadata } from "next";
import { Geist } from "next/font/google";
import { headers } from "next/headers";
import type { ReactNode } from "react";
import { brandIcons } from "@/config/brand";

import { ThemeBootstrapScript } from "@/features/theme/ThemeBootstrapScript";
import { RouteLocaleProvider } from "@/i18n/RouteLocaleProvider";
import { getLocaleDirection } from "@/i18n/locales";
import { getRequestDictionary, getRequestLocale } from "@/i18n/server";
import { getRequestReleaseState } from "@/features/release/server";
import { ReleaseViewProvider } from "@/features/release/ReleaseViewProvider";

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
  const release = await getRequestReleaseState();
  let developmentTools: ReactNode = null;
  if (release.canSwitch) {
    const { LocalDevelopmentTools } = await import("@/features/local-development/LocalDevelopmentTools");
    let seed;
    if (release.localSimulation) {
      const { localWorkspacePreviewConversations, localWorkspacePreviewConversationMessages, localWorkspacePreviewImageModels } = await import("@/features/session/local-workspace-preview");
      const { previewWithTombstones } = await import("@/features/local-development/preview-artifact-deletion.server");
      seed = { conversations: localWorkspacePreviewConversations, messages: await previewWithTombstones(localWorkspacePreviewConversationMessages), imageModels: localWorkspacePreviewImageModels.items };
    }
    developmentTools = <LocalDevelopmentTools seed={seed} />;
  }

  return (
    <html className={geistSans.variable} data-theme="system" dir={getLocaleDirection(locale)} lang={locale} suppressHydrationWarning>
      <head>
        <ThemeBootstrapScript nonce={nonce} />
      </head>
      <body><RouteLocaleProvider locale={locale}><ReleaseViewProvider initial={release}>{children}{developmentTools}</ReleaseViewProvider></RouteLocaleProvider></body>
    </html>
  );
}
