import type { Metadata } from "next";
import { ImageToolLanding } from "@/features/marketing/ImageToolLanding";
import { getRequestDictionary } from "@/i18n/server";

// A visual demonstration, not an admitted/indexable public SEO release.
export async function generateMetadata(): Promise<Metadata> {
  const { imageLanding: t } = await getRequestDictionary();
  return { title: t.metadataTitle, description: t.metadataDescription, robots: { index: false, follow: false } };
}

export default async function ImageGeneratorExamplePage() {
  return <ImageToolLanding />;
}
