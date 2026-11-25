import { brandName } from "@/config/brand";
import { loadPresentationAccess, presentationApiReachable } from "@/features/presentations/access.server";
import { PresentationsPage } from "@/features/presentations/PresentationsPage";
import { parseDeckRun } from "@/features/presentations/deck-run";
import { getRequestReleaseState } from "@/features/release/server";
import { presentationPageCopy } from "@/i18n/presentation-studio";
import { homeEn } from "@/i18n/public/home-en";
import { localizePublicData } from "@/i18n/public/public-copy";
import { publicPageMetadata } from "@/i18n/seo";
import { getRequestLocale, notFound } from "@/i18n/server";
import { presentationWorkflowPublication } from "./publication";

async function requirePreview() {
  const release = await getRequestReleaseState();
  if (release.view !== "dev") notFound();
  return release;
}

export async function generateMetadata() {
  await requirePreview();
  return {
    ...publicPageMetadata(await getRequestLocale(), presentationWorkflowPublication),
    title: `${presentationPageCopy.metadataTitle} | ${brandName}`,
    description: presentationPageCopy.metadataDescription,
  };
}

export default async function Page({ searchParams }: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const release = await requirePreview();
  const copy = localizePublicData(homeEn, await getRequestLocale());
  const access = await loadPresentationAccess();
  const localApiUnavailable = release.localSimulation && access.kind === "guest" && !await presentationApiReachable();
  const run = parseDeckRun(await searchParams);
  return <PresentationsPage copy={copy} access={access} showLocalExample={release.localSimulation}
    localApiUnavailable={localApiUnavailable} run={run} />;
}
