import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

import { PublicHeader } from "../PublicHeader/PublicHeader";
import { HeroComposer } from "../HeroComposer/HeroComposer";
import { LandingToolSelectionProvider } from "../LandingToolSelection/LandingToolSelection";
import { QuickTools } from "./QuickTools";

describe("QuickTools", () => {
  afterEach(() => cleanup());

  it("renders seven tools and explicitly synchronizes the header and composer", () => {
    render(
      <LandingToolSelectionProvider>
        <PublicHeader />
        <HeroComposer />
        <QuickTools />
      </LandingToolSelectionProvider>,
    );

    expect(screen.getAllByTestId("quick-tool")).toHaveLength(7);
    fireEvent.click(screen.getByRole("button", { name: /GPT Image/ }));

    expect(screen.getByLabelText("Выбранный инструмент")).toHaveTextContent("GPT Image");
    expect(screen.getByRole("textbox", { name: "Описание изображения" })).toBeInTheDocument();
  });

  it("keeps the catalog destination as a normal link", () => {
    render(<LandingToolSelectionProvider><QuickTools /></LandingToolSelectionProvider>);

    expect(screen.getByRole("link", { name: /Все нейросети/ })).toHaveAttribute("href", "/login?next=/app/models");
  });
});
