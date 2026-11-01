import { connection } from "next/server";
import { contentMetadata, renderContentPage } from "@/content/templates/render";
import { loadHomeEntrySession } from "@/features/session/home-entry.server";
import { HomeSessionRecovery } from "@/features/session/HomeSessionRecovery";
import { getRequestLocale, redirectLocalized } from "@/i18n/server";

export async function generateMetadata() {
  return contentMetadata("/");
}

export default async function HomePage() {
  await connection();
  const session = await loadHomeEntrySession();
  if (session === "authenticated") redirectLocalized("/app", await getRequestLocale());
  const landing = await renderContentPage("/");
  return session === "refresh_required" ? <>{landing}<HomeSessionRecovery /></> : landing;
}
