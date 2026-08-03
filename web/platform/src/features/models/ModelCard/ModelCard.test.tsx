import type { ReactNode } from "react";

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/link", () => ({
  default: ({ children, href, prefetch, ...props }: { children: ReactNode; href: string; prefetch?: boolean }) => (
    <a data-prefetch={String(prefetch)} href={href} {...props}>
      {children}
    </a>
  ),
}));

import { ru } from "@/i18n/ru";

import { ModelCard } from "./ModelCard";

describe("ModelCard", () => {
  afterEach(() => cleanup());

  it("links a safe model card to the selected generator", () => {
    render(
      <ModelCard
        model={{
          default_quality: "1K",
          id: "nano-banana-2",
          max_reference_images: 1,
          name: "Nano Banana",
          quality_options: ["1K", "2K"],
          supports_reference_image: true,
        }}
      />,
    );

    expect(
      screen.getByRole("link", { name: `${ru.modelsCatalog.openGeneratorLabel}: Nano Banana` }),
    ).toHaveAttribute("href", "/app/image?model=nano-banana-2");
    expect(screen.queryByText(/provider|price|description/i)).not.toBeInTheDocument();
  });

  it("does not prefetch the model-specific generator route", () => {
    render(
      <ModelCard
        model={{
          default_quality: "1K",
          id: "nano-banana-2",
          max_reference_images: 1,
          name: "Nano Banana",
          quality_options: ["1K", "2K"],
          supports_reference_image: true,
        }}
      />,
    );

    expect(
      screen.getByRole("link", { name: `${ru.modelsCatalog.openGeneratorLabel}: Nano Banana` }),
    ).toHaveAttribute("data-prefetch", "false");
  });

  it("shows the lowest verified quality price and omits pricing without API data", () => {
    const { rerender } = render(
      <ModelCard
        model={{
          default_quality: "1K",
          id: "nano-banana-2",
          max_reference_images: 1,
          name: "Nano Banana",
          price_by_quality: { "1K": 16, "2K": 60 },
          quality_options: ["1K", "2K"],
          supports_reference_image: true,
        }}
      />,
    );

    expect(screen.getByText("От 16 ★")).toBeInTheDocument();
    expect(screen.queryByText("От 60 ★")).not.toBeInTheDocument();

    rerender(
      <ModelCard
        model={{
          default_quality: "1K",
          id: "price-not-published",
          max_reference_images: 0,
          name: "No published price",
          quality_options: ["1K"],
          supports_reference_image: false,
        }}
      />,
    );

    expect(screen.queryByText(/★/)).not.toBeInTheDocument();
  });
});
