import { landingFaq } from "../landing-content";

export const publicSiteOrigin = "https://neiirohub.ru";

export function createLandingJsonLd() {
  return [
    {
      "@context": "https://schema.org",
      "@type": "Organization",
      name: "NeiroHub",
      url: `${publicSiteOrigin}/`,
    },
    {
      "@context": "https://schema.org",
      "@type": "WebSite",
      name: "NeiroHub",
      url: `${publicSiteOrigin}/`,
      inLanguage: "ru-RU",
    },
    {
      "@context": "https://schema.org",
      "@type": "FAQPage",
      mainEntity: landingFaq.map((item) => ({
        "@type": "Question",
        name: item.question,
        acceptedAnswer: {
          "@type": "Answer",
          text: item.answer,
        },
      })),
    },
  ] as const;
}

export function serializeJsonLd(value: unknown): string {
  return JSON.stringify(value).replace(/</g, "\\u003c");
}
