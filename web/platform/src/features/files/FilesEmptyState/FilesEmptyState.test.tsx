import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { FilesEmptyState } from "./FilesEmptyState";

describe("FilesEmptyState", () => {
  afterEach(cleanup);

  it("uses the shared neon folder illustration instead of the legacy inline icon", () => {
    const { container } = render(
      <FilesEmptyState description="Описание" title="Пока ничего нет" />,
    );

    const illustration = container.querySelector("img");

    expect(decodeURIComponent(illustration?.getAttribute("src") ?? "")).toContain(
      "/assets/illustrations/files-empty-folder.png",
    );
    expect(illustration).toHaveAttribute("aria-hidden", "true");
    expect(container.querySelector("svg")).not.toBeInTheDocument();
  });
});
