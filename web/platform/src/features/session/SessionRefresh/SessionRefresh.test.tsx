import { render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(),
}));

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserMutation: vi.fn(),
}));

import { useRouter } from "next/navigation";

import { webBrowserMutation } from "@/lib/web-api/browser";

import { SessionRefresh } from "./SessionRefresh";

const refresh = vi.fn();
const replace = vi.fn();

describe("SessionRefresh", () => {
  beforeEach(() => {
    vi.mocked(useRouter).mockReturnValue({ refresh, replace } as never);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("makes one browser refresh mutation and reloads the server tree only after 200", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 200 }));

    render(<SessionRefresh />);

    await vi.waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
    expect(webBrowserMutation).toHaveBeenCalledTimes(1);
    expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/auth/refresh", { method: "POST" });
    expect(replace).not.toHaveBeenCalled();
  });

  it.each([
    ["a non-200 response", () => Promise.resolve(new Response(null, { status: 401 }))],
    ["a rejected refresh request", () => Promise.reject(new Error("untrusted failure"))],
    ["a missing CSRF helper rejection", () => Promise.reject(new Error("Unable to complete the request."))],
  ])("redirects to login after %s", async (_caseName, request) => {
    vi.mocked(webBrowserMutation).mockImplementationOnce(request);

    render(<SessionRefresh />);

    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith("/login"));
    expect(refresh).not.toHaveBeenCalled();
  });

  it("does not retry the refresh mutation on re-render", async () => {
    let resolveRequest: (response: Response) => void = () => undefined;
    vi.mocked(webBrowserMutation).mockReturnValue(
      new Promise<Response>((resolve) => {
        resolveRequest = resolve;
      }),
    );

    const view = render(<SessionRefresh />);
    view.rerender(<SessionRefresh />);

    expect(webBrowserMutation).toHaveBeenCalledTimes(1);
    resolveRequest(new Response(null, { status: 200 }));
    await vi.waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
  });
});
