import type { Metadata } from "next";
import { ImageToolLanding } from "@/features/marketing/ImageToolLanding";
import { getRequestDictionary, getRequestLocale } from "@/i18n/server";
import { publicPageMetadata } from "@/i18n/seo";
import { imageGeneratorPublication } from "./publication";

// A visual demonstration, not an admitted/indexable public SEO release.
export async function generateMetadata(): Promise<Metadata> {
  const { imageLanding: t } = await getRequestDictionary();
  return { ...publicPageMetadata(await getRequestLocale(), imageGeneratorPublication), title: t.metadataTitle, description: t.metadataDescription };
}

export default async function ImageGeneratorExamplePage() {
  return <ImageToolLanding />;
}
