import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

const route = vi.hoisted(() => ({ pathname: "/" }));
vi.mock("../navigation", () => ({ usePathname: () => route.pathname }));

import { LocaleProvider } from "../LocaleProvider";
import { LocalizedPublicShell } from "./LocalizedPublicShell";

describe("LocalizedPublicShell", () => {
  it("lets the homepage own its chrome instead of rendering duplicate navigation", () => {
    route.pathname = "/";
    render(<LocalizedPublicShell><h1>Homepage</h1></LocalizedPublicShell>);
    expect(screen.getByRole("heading", { name: "Homepage" })).toBeInTheDocument();
    expect(screen.queryByRole("banner")).not.toBeInTheDocument();
    expect(screen.queryByRole("contentinfo")).not.toBeInTheDocument();
  });
  it("preserves shared localized chrome on other public routes", () => {
    route.pathname = "/guides";
    render(<LocaleProvider locale="en"><LocalizedPublicShell><h1>Guide</h1></LocalizedPublicShell></LocaleProvider>);
    expect(screen.getByRole("banner")).toBeInTheDocument();
    expect(screen.getByRole("main")).toContainElement(screen.getByRole("heading", { name: "Guide" }));
    expect(screen.getByRole("link", { name: "Open platform" })).toHaveAttribute("href", "/en/app");
    expect(screen.getByRole("contentinfo")).toBeInTheDocument();
  });
});
