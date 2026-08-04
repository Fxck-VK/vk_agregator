import type { MetadataRoute } from "next";

import { publicSiteOrigin } from "@/features/landing/seo/landing-json-ld";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: "*",
      allow: "/",
      disallow: ["/app/", "/login", "/web/"],
    },
    sitemap: `${publicSiteOrigin}/sitemap.xml`,
    host: publicSiteOrigin,
  };
}
