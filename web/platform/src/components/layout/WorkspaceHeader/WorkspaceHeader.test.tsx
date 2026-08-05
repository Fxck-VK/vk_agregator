import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: vi.fn(),
}));

vi.mock("@/features/models/WorkspaceModelSelector/WorkspaceModelSelector", () => ({
  WorkspaceModelSelector: () => <button type="button">Nano Banana 2</button>,
}));

import { usePathname } from "next/navigation";

import { WorkspaceHeader } from "./WorkspaceHeader";

describe("WorkspaceHeader", () => {
  beforeEach(() => {
    vi.mocked(usePathname).mockReturnValue("/app/profile");
  });

  afterEach(() => {
    cleanup();
  });

  it("names the fixed header after the profile route", () => {
    render(<WorkspaceHeader balance={104} />);

    expect(screen.getByRole("banner", { name: "Профиль" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Nano Banana 2" })).toBeInTheDocument();
    expect(screen.queryByText("Профиль")).toBeNull();
  });

  it("keeps the Inspiration title and excludes the selector from the whole section", () => {
    vi.mocked(usePathname).mockReturnValue("/app/inspiration/example");

    render(<WorkspaceHeader balance={104} />);

    expect(screen.getByRole("banner", { name: "Вдохновение" })).toBeInTheDocument();
    expect(screen.getByText("Вдохновение")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Nano Banana 2" })).toBeNull();
  });

  it("does not treat a similarly prefixed route as the Inspiration section", () => {
    vi.mocked(usePathname).mockReturnValue("/app/inspiration-tools");

    render(<WorkspaceHeader balance={104} />);

    expect(screen.getByRole("button", { name: "Nano Banana 2" })).toBeInTheDocument();
  });
});
