import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { brandMonogram, brandName } from "@/config/brand";
import { brandLogoPath } from "@/config/brand";
import { BrandLogo } from "@/components/brand/BrandLogo/BrandLogo";

import { BrandName, BrandText } from "./BrandName";

describe("shared brand typography", () => {
  it("renders the SVG mark directly without a bitmap optimizer URL", () => {
    const { container } = render(<BrandLogo />);
    const logo = container.querySelector("img[data-brand-logo]");
    expect(logo).toHaveAttribute("src", brandLogoPath);
    expect(logo?.getAttribute("src")).not.toContain("/_next/image");
    expect(logo).toHaveAttribute("width", "40");
    expect(logo).toHaveAttribute("height", "40");
  });
  it("uses the configured SVG lettering while retaining the accessible brand text", () => {
    const { container } = render(<BrandName />);
    const wordmark = container.querySelector("svg[data-brand-wordmark]");
    expect(wordmark).toHaveAttribute("aria-hidden", "true");
    expect(wordmark?.querySelector("use")).toHaveAttribute("href", "/assets/brand/wordmarks/epifera.svg#wordmark");
    expect(container.textContent).toBe(brandName);
  });

  it("keeps the monogram as text", () => {
    const { container } = render(<BrandName monogram />);
    expect(container.querySelector("svg")).toBeNull();
    expect(container.textContent).toBe(brandMonogram);
  });
  it("uses the configured name and the same style for the monogram", () => {
    const { container } = render(<><BrandName /><BrandName monogram /></>);
    const names = container.querySelectorAll("[data-brand-name]");
    expect(names[0]).toHaveTextContent(brandName);
    expect(names[1]).toHaveTextContent(brandMonogram);
    expect(names[0].className).toBe(names[1].className);
  });

  it.each([`Войти в ${brandName}`, `How ${brandName} works`, `${brandName} Chat`, `What are ${brandName}'s own AI models?`])("styles just the brand in %s without changing the readable text", copy => {
    const { container } = render(<h1><BrandText>{copy}</BrandText></h1>);
    expect(screen.getByRole("heading", { name: copy })).toBeInTheDocument();
    expect(container.querySelector("[data-brand-name]")).toHaveTextContent(brandName);
    expect(container.textContent).toBe(copy);
  });

  it("handles repeated names without decorating names embedded in another word", () => {
    const copy = `${brandName}: ${brandName}, X${brandName} ${brandName}x`;
    const { container } = render(<BrandText>{copy}</BrandText>);
    expect(container.querySelectorAll("[data-brand-name]")).toHaveLength(2);
    expect(container.textContent).toBe(copy);
  });

  it("escapes copy as text and leaves provider names alone", () => {
    const { container } = render(<><BrandText>{`<img src=x onerror=alert(1)> ${brandName}`}</BrandText><BrandText>GPT Image 2</BrandText></>);
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelectorAll("[data-brand-name]")).toHaveLength(1);
    expect(container.textContent).toContain("<img src=x onerror=alert(1)>");
    expect(container.textContent).toContain("GPT Image 2");
  });
});
