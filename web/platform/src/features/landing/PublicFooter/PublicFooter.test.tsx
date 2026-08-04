import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { landingFooterGroups } from "../landing-content";
import { PublicFooter } from "./PublicFooter";

describe("PublicFooter", () => {
  it("renders only real link targets with grouped headings", () => {
    render(<PublicFooter />);

    expect(screen.getAllByRole("heading", { level: 2 })).toHaveLength(landingFooterGroups.length);
    for (const link of screen.getAllByRole("link")) {
      expect(link.getAttribute("href")).toMatch(/^\/(?:$|login\?next=\/app)/);
    }
  });
});
