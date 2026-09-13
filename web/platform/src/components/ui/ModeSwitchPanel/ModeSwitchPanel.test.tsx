import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import {
  ModeSwitchPanel,
  type ModeSwitchPanelItem,
} from "./ModeSwitchPanel";

type ModeID = "general" | "animate" | "enhance";

const items: readonly ModeSwitchPanelItem<ModeID>[] = [
  { icon: <span aria-hidden="true">G</span>, id: "general", label: "Общая" },
  { icon: <span aria-hidden="true">A</span>, id: "animate", label: "Оживить" },
  { disabled: true, icon: <span aria-hidden="true">E</span>, id: "enhance", label: "Улучшить" },
];

function ControlledPanel({ iconOnly = false }: { iconOnly?: boolean }) {
  const [activeID, setActiveID] = useState<ModeID>("general");
  return (
    <ModeSwitchPanel
      activeID={activeID}
      ariaLabel="Режим работы"
      className="consumer-width"
      iconOnly={iconOnly}
      items={items}
      onChange={setActiveID}
    />
  );
}

describe("ModeSwitchPanel", () => {
  it("places the initial indicator without a transition and restores motion for selection changes", () => {
    const measure = vi.spyOn(HTMLElement.prototype, "getBoundingClientRect")
      .mockImplementation(function (this: HTMLElement) {
        if (this.getAttribute("role") === "toolbar") {
          return new DOMRect(100, 0, 600, 44);
        }
        if (this.tagName === "BUTTON") {
          return this.textContent?.includes("Общая")
            ? new DOMRect(260, 0, 130, 44)
            : new DOMRect(390, 0, 150, 44);
        }
        return new DOMRect();
      });

    try {
      render(<ControlledPanel />);

      const indicator = screen.getByTestId("mode-switch-panel-indicator");
      expect(indicator).toHaveAttribute("data-ready", "true");
      expect(indicator).toHaveStyle({
        "inline-size": "130px",
        transform: "translate3d(160px, 0, 0)",
        transition: "none",
      });

      fireEvent.click(screen.getByRole("button", { name: "Оживить" }));

      expect(indicator).toHaveStyle({
        "inline-size": "150px",
        transform: "translate3d(290px, 0, 0)",
      });
      expect(indicator.style.transition).toBe("");
    } finally {
      measure.mockRestore();
    }
  });

  it("renders a consumer-defined set of modes and delegates selection", () => {
    const onChange = vi.fn();
    render(
      <ModeSwitchPanel
        activeID="general"
        ariaLabel="Режим работы"
        className="consumer-width"
        items={items}
        onChange={onChange}
      />,
    );

    const toolbar = screen.getByRole("toolbar", { name: "Режим работы" });
    expect(toolbar.closest(".consumer-width")).not.toBeNull();
    expect(screen.getAllByRole("button")).toHaveLength(items.length);
    expect(screen.getByRole("button", { name: "Общая" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Улучшить" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Оживить" }));

    expect(onChange).toHaveBeenCalledWith("animate");
  });

  it("owns roving keyboard navigation and skips disabled modes", () => {
    render(<ControlledPanel />);

    const general = screen.getByRole("button", { name: "Общая" });
    const animate = screen.getByRole("button", { name: "Оживить" });

    general.focus();
    fireEvent.keyDown(general, { key: "ArrowLeft" });
    expect(animate).toHaveFocus();
    expect(animate).toHaveAttribute("aria-pressed", "true");

    fireEvent.keyDown(animate, { key: "ArrowRight" });
    expect(general).toHaveFocus();
    expect(general).toHaveAttribute("aria-pressed", "true");
  });

  it("keeps icon-only modes named and keyboard-operable without visible labels", () => {
    render(<ControlledPanel iconOnly />);

    const general = screen.getByRole("button", { name: "Общая" });
    const animate = screen.getByRole("button", { name: "Оживить" });
    expect(screen.queryByText("Общая")).not.toBeInTheDocument();
    expect(screen.queryByText("Оживить")).not.toBeInTheDocument();

    general.focus();
    fireEvent.keyDown(general, { key: "ArrowRight" });

    expect(animate).toHaveFocus();
    expect(animate).toHaveAttribute("aria-pressed", "true");
    expect(general).toHaveAttribute("aria-pressed", "false");
  });

  it("supports iconless tabs linked to an external panel", () => {
    render(
      <ModeSwitchPanel
        activeID="general"
        ariaLabel="Категории"
        items={[
          {
            ariaControls: "catalog-panel",
            elementID: "category-general",
            id: "general",
            label: "Общая",
          },
          {
            ariaControls: "catalog-panel",
            elementID: "category-animate",
            id: "animate",
            label: "Оживить",
          },
        ]}
        onChange={() => {}}
        semantics="tabs"
      />,
    );

    const tablist = screen.getByRole("tablist", { name: "Категории" });
    const selectedTab = screen.getByRole("tab", { name: "Общая" });
    expect(tablist.closest('[data-mode-switch-panel="true"]')).not.toBeNull();
    expect(selectedTab).toHaveAttribute("aria-selected", "true");
    expect(selectedTab).toHaveAttribute("aria-controls", "catalog-panel");
    expect(selectedTab).toHaveAttribute("id", "category-general");
    expect(selectedTab.querySelector("[aria-hidden=true]")).toBeNull();
  });
});
