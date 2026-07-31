import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/server", () => ({
  connection: vi.fn(),
}));

import { connection } from "next/server";

import HomePage from "./page";

describe("HomePage", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("waits for the request so Next can attach the per-request CSP nonce", async () => {
    vi.mocked(connection).mockResolvedValue(undefined);

    const markup = renderToStaticMarkup(await HomePage());

    expect(connection).toHaveBeenCalledOnce();
    expect(markup).toContain("NeiroHub");
  });
});
