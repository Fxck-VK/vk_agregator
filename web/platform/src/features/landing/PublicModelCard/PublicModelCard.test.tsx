import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { landingModels } from "../landing-content";
import { PublicModelCard } from "./PublicModelCard";

describe("PublicModelCard", () => {
  it("renders a safe target without invented ratings or counters", () => {
    render(<PublicModelCard model={landingModels[0]} />);

    expect(screen.getByRole("link", { name: /NeiroHub/ })).toHaveAttribute(
      "href",
      "/login?next=/app/chats",
    );
    expect(screen.queryByText(/рейтинг/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/пользовател/i)).not.toBeInTheDocument();
  });
});
