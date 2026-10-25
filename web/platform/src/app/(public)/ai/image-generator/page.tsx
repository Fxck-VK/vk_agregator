import {localizePublicData} from "@/i18n/public/public-copy";
import type { Metadata } from "next";
import { ImageGeneratorLanding } from "@/features/image-generator-landing/ImageGeneratorLanding";
import { imageGeneratorEn } from "@/i18n/public/image-generator-en";
import { getRequestLocale } from "@/i18n/server";
import { publicPageMetadata } from "@/i18n/seo";
import { imageGeneratorPublication } from "./publication";

// A draft visual rebuild, not an admitted/indexable public SEO release.
export async function generateMetadata(): Promise<Metadata> {
  return { ...publicPageMetadata(await getRequestLocale(), imageGeneratorPublication), ...localizePublicData(imageGeneratorEn.metadata,await getRequestLocale()) };
}

export default async function ImageGeneratorPage() {
  return <ImageGeneratorLanding />;
}
