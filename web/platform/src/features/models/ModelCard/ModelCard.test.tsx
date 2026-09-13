import type { ReactNode } from "react";

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/link", () => ({
  default: ({ children, href, prefetch, ...props }: { children: ReactNode; href: string; prefetch?: boolean }) => (
    <a data-prefetch={String(prefetch)} href={href} {...props}>
      {children}
    </a>
  ),
}));

import { ru } from "@/i18n/ru";
import selectableStyles from "@/components/ui/selectable-control.module.css";

import { ModelCard } from "./ModelCard";

describe("ModelCard", () => {
  afterEach(() => cleanup());

  it("renders the approved shared name and description without catalogue-only facts", () => {
    render(
      <ModelCard
        model={{
          default_quality: "1K",
          id: "nano-banana-2",
          max_reference_images: 1,
          name: "Nano Banana 2",
          quality_options: ["1K", "2K"],
          supports_reference_image: true,
        }}
      />,
    );

    const heading = screen.getByRole("heading", { level: 3, name: "Nano Banana 2" });
    const description = screen.getByText(
      "Быстрая генерация и редактирование изображений для повседневных задач",
    );

    expect(heading).toBeInTheDocument();
    expect(
      heading.compareDocumentPosition(description) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(screen.queryByRole("list", { name: ru.modelsCatalog.qualityFilterLabel })).toBeNull();
    expect(screen.queryByText(ru.modelsCatalog.referenceSupportedLabel)).toBeNull();
  });

  it("accepts section-level animation hooks without changing the shared card", () => {
    render(
      <ModelCard
        className="section-reveal"
        model={{
          default_quality: "1K",
          id: "nano-banana-2",
          max_reference_images: 1,
          name: "Nano Banana 2",
          quality_options: ["1K"],
          supports_reference_image: true,
        }}
        revealed
        testId="featured-model-card"
      />,
    );

    expect(screen.getByTestId("featured-model-card")).toHaveClass("section-reveal");
    expect(screen.getByTestId("featured-model-card")).toHaveAttribute("data-revealed", "true");
  });

  it("uses the shared model fallback artwork", () => {
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

    expect(screen.getByTestId("model-icon-fallback")).toBeInTheDocument();
    expect(screen.queryByTestId("model-icon")).not.toBeInTheDocument();
  });

  it("renders a compact selectable variant with the same content and canonical target", () => {
    const onActivate = vi.fn();
    const model = {
      default_quality: "1K",
      id: "nano / banana",
      max_reference_images: 1,
      name: "Nano / Banana",
      quality_options: ["1K"],
      supports_reference_image: true,
    };

    render(
      <ModelCard
        model={model}
        onActivate={onActivate}
        selected
        variant="selector"
      />,
    );

    const card = screen.getByRole("button", { name: /Nano \/ Banana/ });
    expect(card).toHaveAttribute("aria-pressed", "true");
    expect(card).toHaveTextContent(
      "Nano / Banana для создания изображений по вашему описанию",
    );
    expect(screen.queryByTestId("credit-star-icon")).not.toBeInTheDocument();
    expect(card).toHaveClass(selectableStyles.control);

    fireEvent.click(card);
    expect(onActivate).toHaveBeenCalledExactlyOnceWith(
      model,
      "/app/image?model=nano%20%2F%20banana",
    );
  });

  it("makes the whole safe model card a generator link without a separate action", () => {
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

    expect(screen.getByRole("link", { name: /Nano Banana/i })).toHaveAttribute(
      "href",
      "/app/image?model=nano-banana-2",
    );
    expect(screen.queryByText(ru.modelsCatalog.openGeneratorLabel)).not.toBeInTheDocument();
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

    expect(screen.getByRole("link", { name: /Nano Banana/i })).toHaveAttribute(
      "data-prefetch",
      "false",
    );
  });

  it("renders a catalogue placeholder with the shared card presentation but no generator link", () => {
    render(
      <ModelCard
        interactive={false}
        model={{
          default_quality: "1K",
          id: "catalog-preview-recraft-v3",
          max_reference_images: 0,
          name: "Recraft V3",
          price_by_quality: { "1K": 35 },
          quality_options: ["1K"],
          supports_reference_image: false,
        }}
      />,
    );

    expect(screen.getByRole("heading", { name: "Recraft V3" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Recraft V3/ })).not.toBeInTheDocument();
    expect(screen.getByTestId("catalog-placeholder-card")).toHaveAttribute(
      "data-interactive",
      "false",
    );
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

    expect(screen.getByLabelText("16 звёзд")).toBeInTheDocument();
    expect(screen.queryByLabelText("60 звёзд")).not.toBeInTheDocument();

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

    expect(screen.queryByTestId("credit-star-icon")).not.toBeInTheDocument();
  });
});
