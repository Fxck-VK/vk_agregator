import { getRequestReleaseState } from "@/features/release/server";
import { parseDeckRun } from "@/features/presentations/deck-run";
import { PresentationWorkspace } from "@/features/presentations/PresentationWorkspace";
import { notFound } from "@/i18n/server";

export default async function Page({ searchParams }: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const release = await getRequestReleaseState();
  if (release.view !== "dev") notFound();
  return <PresentationWorkspace run={parseDeckRun(await searchParams)} preview={release.localSimulation} />;
}
