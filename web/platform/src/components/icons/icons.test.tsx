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
    ["edit", EditIcon, "M 414 622"],
    ["file", FileIcon, "M 419 340"],
    ["grid", GridIcon, "M 661 676"],
    ["image", ImageIcon, "M 932 392"],
  ] as const)("renders the %s navigation icon using the approved theme-aware artwork", (name, Icon, pathPrefix) => {
    render(<Icon data-testid={`${name}-icon`} />);

    const icon = screen.getByTestId(`${name}-icon`);
    expect(icon).toHaveAttribute("aria-hidden", "true");
    expect(icon).toHaveAttribute("data-icon", name);
    expect(icon).toHaveAttribute("fill", "none");
    expect(icon).toHaveAttribute("viewBox", "0 0 1254 1254");

    const paths = icon.querySelectorAll("path");
    expect(paths).toHaveLength(1);
    expect(paths[0]).toHaveAttribute("fill", "currentColor");
    expect(paths[0]?.getAttribute("d")).toMatch(new RegExp(`^${pathPrefix}`));
  });
});
