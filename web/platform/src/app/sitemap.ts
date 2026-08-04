import type { MetadataRoute } from "next";

import { publicSiteOrigin } from "@/features/landing/seo/landing-json-ld";

export default function sitemap(): MetadataRoute.Sitemap {
  return [
    {
      url: `${publicSiteOrigin}/`,
      changeFrequency: "weekly",
      priority: 1,
    },
  ];
}
