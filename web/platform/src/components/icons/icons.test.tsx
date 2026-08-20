import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { assetPaths } from "@/assets/asset-paths";

import { CheckIcon } from "./CheckIcon";
import { CopyIcon } from "./CopyIcon";
import { EditIcon } from "./EditIcon";
import { FileIcon } from "./FileIcon";
import { GridIcon } from "./GridIcon";
import { ImageIcon } from "./ImageIcon";
import { MegaphoneIcon } from "./MegaphoneIcon";
import { MonitorIcon } from "./MonitorIcon";
import { MoonIcon } from "./MoonIcon";
import { ProfileIcon } from "./ProfileIcon";
import { SunIcon } from "./SunIcon";
import { SupportIcon } from "./SupportIcon";

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
    ["edit", EditIcon, "M 414 622", "300 277 675 675"],
    ["file", FileIcon, "M 419 340", "300 300 654 654"],
    ["grid", GridIcon, "M 661 676", "336 336 581 581"],
    ["image", ImageIcon, "M 932 392", "276 272 701 701"],
  ] as const)("renders the %s navigation icon using tightly fitted approved artwork", (name, Icon, pathPrefix, viewBox) => {
    render(<Icon data-testid={`${name}-icon`} />);

    const icon = screen.getByTestId(`${name}-icon`);
    expect(icon).toHaveAttribute("aria-hidden", "true");
    expect(icon).toHaveAttribute("data-icon", name);
    expect(icon).toHaveAttribute("fill", "none");
    expect(icon).toHaveAttribute("viewBox", viewBox);

    const paths = icon.querySelectorAll("path");
    expect(paths).toHaveLength(1);
    expect(paths[0]).toHaveAttribute("fill", "currentColor");
    expect(paths[0]?.getAttribute("d")).toMatch(new RegExp(`^${pathPrefix}`));
  });

  it.each([
    ["profile", ProfileIcon, assetPaths.icons.accountMenu.profile],
    ["support", SupportIcon, assetPaths.icons.accountMenu.support],
    ["megaphone", MegaphoneIcon, assetPaths.icons.accountMenu.megaphone],
  ] as const)("renders the approved %s account-menu artwork through the shared asset icon", (name, Icon, source) => {
    render(<Icon data-testid={`${name}-icon`} />);

    const icon = screen.getByTestId(`${name}-icon`);
    expect(icon).toHaveAttribute("aria-hidden", "true");
    expect(icon).toHaveAttribute("data-icon", name);
    expect(icon.style.getPropertyValue("--asset-icon-source")).toBe(`url("${source}")`);
  });

  it.each([
    ["monitor", MonitorIcon, assetPaths.icons.theme.monitor],
    ["sun", SunIcon, assetPaths.icons.theme.sun],
    ["moon", MoonIcon, assetPaths.icons.theme.moon],
  ] as const)("renders the approved %s theme artwork through the shared asset icon", (name, Icon, source) => {
    render(<Icon data-testid={`${name}-icon`} />);

    const icon = screen.getByTestId(`${name}-icon`);
    expect(icon).toHaveAttribute("aria-hidden", "true");
    expect(icon).toHaveAttribute("data-icon", name);
    expect(icon.style.getPropertyValue("--asset-icon-source")).toBe(`url("${source}")`);
  });
});
