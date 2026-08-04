import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { landingFaq } from "../landing-content";
import { FaqSection } from "./FaqSection";

describe("FaqSection", () => {
  it("server-renders every answer with a native no-JavaScript accordion fallback", () => {
    const markup = renderToStaticMarkup(<FaqSection />);
    const document = new DOMParser().parseFromString(markup, "text/html");
    const details = Array.from(document.querySelectorAll("details"));

    expect(details).toHaveLength(landingFaq.length);
    expect(details[0]?.hasAttribute("open")).toBe(true);
    expect(details.every((item) => item.getAttribute("name") === "landing-faq")).toBe(true);
    for (const item of landingFaq) {
      expect(markup).toContain(item.question);
      expect(markup).toContain(item.answer);
    }
  });
});
