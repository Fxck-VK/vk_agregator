import { describe, expect, it } from "vitest";

import { landingFaq } from "../landing-content";
import { createLandingJsonLd, serializeJsonLd } from "./landing-json-ld";

describe("landing JSON-LD", () => {
  it("builds organization, website and FAQ schemas from visible content", () => {
    const schemas = createLandingJsonLd();

    expect(schemas.map(({ "@type": type }) => type)).toEqual(["Organization", "WebSite", "FAQPage"]);
    expect(schemas[2]).toMatchObject({
      mainEntity: landingFaq.map((item) => ({
        "@type": "Question",
        name: item.question,
        acceptedAnswer: {
          "@type": "Answer",
          text: item.answer,
        },
      })),
    });
  });

  it("escapes markup-significant characters before embedding JSON in a script", () => {
    expect(serializeJsonLd({ value: "</script><script>alert(1)</script>" })).not.toContain("<");
    expect(JSON.parse(serializeJsonLd({ value: "</script><script>alert(1)</script>" }))).toEqual({
      value: "</script><script>alert(1)</script>",
    });
  });
});
