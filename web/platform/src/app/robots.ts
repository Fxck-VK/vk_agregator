import type { MetadataRoute } from "next";
import { getPublicOrigin } from "@/i18n/seo";

export const dynamic = "force-dynamic";
export default function robots(): MetadataRoute.Robots {
  return {
    rules: { userAgent: "*", allow: "/", disallow: ["/web/", "/api/"] },
    sitemap: `${getPublicOrigin()}/sitemap.xml`,
  };
}
