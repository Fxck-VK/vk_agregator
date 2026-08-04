import type { Metadata } from "next";

import { PublicHome } from "@/features/landing/PublicHome/PublicHome";
import { PublicShell } from "@/features/landing/PublicShell/PublicShell";
import { createLandingJsonLd, serializeJsonLd } from "@/features/landing/seo/landing-json-ld";

export const metadata: Metadata = {
  title: "Нейросети онлайн на русском — NeiroHub",
  description: "Общайтесь с нейросетями, создавайте изображения и решайте рабочие задачи на русском языке в единой платформе NeiroHub.",
  alternates: {
    canonical: "/",
  },
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
      "max-image-preview": "large",
      "max-snippet": -1,
      "max-video-preview": -1,
    },
  },
  openGraph: {
    type: "website",
    url: "/",
    locale: "ru_RU",
    siteName: "NeiroHub",
    title: "Нейросети онлайн на русском — NeiroHub",
    description: "Единая платформа для общения с нейросетями и создания контента.",
    images: [
      {
        url: "/inspiration/paper-crane-cloud.png",
        alt: "NeiroHub — нейросети и создание контента в одном месте",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "Нейросети онлайн на русском — NeiroHub",
    description: "Единая платформа для общения с нейросетями и создания контента.",
    images: ["/inspiration/paper-crane-cloud.png"],
  },
};

export default function HomePage() {
  return (
    <>
      <PublicShell>
        <PublicHome />
      </PublicShell>
      <script
        dangerouslySetInnerHTML={{ __html: serializeJsonLd(createLandingJsonLd()) }}
        type="application/ld+json"
      />
    </>
  );
}
