import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PublicHome, publicHomeBlockOrder } from "./PublicHome";

describe("PublicHome", () => {
  it("renders one H1 and the approved twelve blocks in order", () => {
    const markup = renderToStaticMarkup(<PublicHome />);
    const document = new DOMParser().parseFromString(markup, "text/html");
    const blocks = Array.from(document.querySelectorAll<HTMLElement>("[data-landing-block]"));

    expect(document.querySelectorAll("h1")).toHaveLength(1);
    expect(document.querySelector("h1")?.textContent).toBe("Простой старт в мир нейросетей");
    expect(blocks.map((block) => block.dataset.landingBlock)).toEqual(publicHomeBlockOrder);
  });

  it("server-renders critical models, FAQ and footer content", () => {
    const markup = renderToStaticMarkup(<PublicHome />);

    expect(markup).toContain("Более 90 нейросетей");
    expect(markup).toContain("Что такое NeiroHub?");
    expect(markup).toContain("Генерация изображений");
  });
});
