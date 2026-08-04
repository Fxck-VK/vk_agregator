import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PublicShell } from "./PublicShell";

describe("PublicShell", () => {
  it("provides a skip link and a single public main region", () => {
    const markup = renderToStaticMarkup(
      <PublicShell>
        <section>Публичный контент</section>
      </PublicShell>,
    );
    const document = new DOMParser().parseFromString(markup, "text/html");

    expect(document.querySelector('a[href="#public-main"]')?.textContent).toBe("Перейти к основному содержимому");
    expect(document.querySelectorAll("main#public-main")).toHaveLength(1);
    expect(document.querySelector("main")?.textContent).toContain("Публичный контент");
    expect(markup).not.toContain("Недавние чаты");
  });
});
