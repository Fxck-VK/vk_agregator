import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: vi.fn(),
}));

import { usePathname } from "next/navigation";

import { WorkspaceHeader } from "./WorkspaceHeader";

describe("WorkspaceHeader", () => {
  beforeEach(() => {
    vi.mocked(usePathname).mockReturnValue("/app/profile");
  });

  it("names the fixed header after the profile route", () => {
    render(<WorkspaceHeader balance={104} />);

    expect(screen.getByRole("banner", { name: "Профиль" })).toBeInTheDocument();
  });
});
