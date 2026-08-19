import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { CheckIcon } from "./CheckIcon";
import { CopyIcon } from "./CopyIcon";
import { EditIcon } from "./EditIcon";
import { FileIcon } from "./FileIcon";
import { GridIcon } from "./GridIcon";
import { ImageIcon } from "./ImageIcon";

describe("shared icons", () => {
  it("renders the copy icon as a decorative, theme-aware SVG", () => {
    render(<CopyIcon data-testid="copy-icon" />);

    const icon = screen.getByTestId("copy-icon");
    expect(icon).toHaveAttribute("aria-hidden", "true");
    expect(icon).toHaveAttribute("data-icon", "copy");
    expect(icon).toHaveAttribute("fill", "none");
  });

  it("renders the check icon as a decorative, theme-aware SVG", () => {
    render(<CheckIcon data-testid="check-icon" />);

    const icon = screen.getByTestId("check-icon");
    expect(icon).toHaveAttribute("aria-hidden", "true");
    expect(icon).toHaveAttribute("data-icon", "check");
    expect(icon).toHaveAttribute("fill", "none");
  });

  it.each([
    ["edit", EditIcon],
    ["file", FileIcon],
    ["grid", GridIcon],
    ["image", ImageIcon],
  ] as const)("renders the %s navigation icon as a decorative, theme-aware SVG", (name, Icon) => {
    render(<Icon data-testid={`${name}-icon`} />);

    const icon = screen.getByTestId(`${name}-icon`);
    expect(icon).toHaveAttribute("aria-hidden", "true");
    expect(icon).toHaveAttribute("data-icon", name);
    expect(icon).toHaveAttribute("fill", "none");
  });
});
