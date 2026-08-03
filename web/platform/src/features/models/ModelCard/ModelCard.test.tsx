import type { ReactNode } from "react";

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

import { ru } from "@/i18n/ru";

import { ModelCard } from "./ModelCard";

describe("ModelCard", () => {
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
});
