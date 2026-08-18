import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SessionProgressBar } from "./SessionProgressBar";

describe("SessionProgressBar", () => {
  it("does not expose a progressbar before the delayed state", () => {
    render(<SessionProgressBar label="Восстановление сессии" visible={false} />);

    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
  });

  it("exposes an accessible progressbar when visible", () => {
    render(<SessionProgressBar label="Восстановление сессии" visible />);

    expect(screen.getByRole("progressbar", { name: "Восстановление сессии" })).toBeInTheDocument();
  });
});
