import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(),
}));

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserMutation: vi.fn(),
}));

import { useRouter } from "next/navigation";

import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";

import { SessionRefresh } from "./SessionRefresh";

const refresh = vi.fn();
const replace = vi.fn();

describe("SessionRefresh", () => {
  beforeEach(() => {
    vi.mocked(useRouter).mockReturnValue({ refresh, replace } as never);
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it("refreshes the server tree only after a successful session refresh", async () => {
    vi.mocked(webBrowserMutation).mockResolvedValue(
      new Response(null, { status: 200 }),
    );

    render(<SessionRefresh />);

    await vi.waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
    expect(webBrowserMutation).toHaveBeenCalledTimes(1);
    expect(webBrowserMutation).toHaveBeenCalledWith(
      "/web/v1/auth/refresh",
      expect.objectContaining({
        method: "POST",
        signal: expect.any(AbortSignal),
      }),
    );
    expect(replace).not.toHaveBeenCalled();
  });

  it("shows the top progress line only after 150 milliseconds", async () => {
    vi.useFakeTimers();
    vi.mocked(webBrowserMutation).mockReturnValue(new Promise(() => undefined));

    render(<SessionRefresh />);

    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    await act(async () => {
      vi.advanceTimersByTime(149);
    });
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();

    await act(async () => {
      vi.advanceTimersByTime(1);
    });
    expect(
      screen.getByRole("progressbar", {
        name: ru.workspace.sessionProgressLabel,
      }),
    ).toBeInTheDocument();
  });

  it.each([400, 401, 403])(
    "redirects to login when refresh returns %s",
    async (status) => {
      vi.mocked(webBrowserMutation).mockResolvedValue(
        new Response(null, { status }),
      );

      render(<SessionRefresh />);

      await vi.waitFor(() => expect(replace).toHaveBeenCalledWith("/login"));
      expect(refresh).not.toHaveBeenCalled();
    },
  );

  it.each([408, 429, 500, 503])(
    "keeps the user in place and offers retry when refresh returns %s",
    async (status) => {
      vi.mocked(webBrowserMutation).mockResolvedValue(
        new Response(null, { status }),
      );

      render(<SessionRefresh />);

      expect(
        await screen.findByRole("button", { name: ru.workspace.sessionRetry }),
      ).toBeInTheDocument();
      expect(replace).not.toHaveBeenCalled();
      expect(refresh).not.toHaveBeenCalled();
      expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    },
  );

  it("keeps the user in place after a network rejection", async () => {
    vi.mocked(webBrowserMutation).mockRejectedValue(
      new TypeError("Failed to fetch"),
    );

    render(<SessionRefresh />);

    expect(
      await screen.findByRole("button", { name: ru.workspace.sessionRetry }),
    ).toBeInTheDocument();
    expect(replace).not.toHaveBeenCalled();
  });

  it("retries in place and refreshes after the next successful response", async () => {
    vi.mocked(webBrowserMutation)
      .mockResolvedValueOnce(new Response(null, { status: 503 }))
      .mockResolvedValueOnce(new Response(null, { status: 200 }));

    render(<SessionRefresh />);

    fireEvent.click(
      await screen.findByRole("button", { name: ru.workspace.sessionRetry }),
    );

    await vi.waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
    expect(webBrowserMutation).toHaveBeenCalledTimes(2);
    expect(replace).not.toHaveBeenCalled();
  });

  it("times out a stalled refresh and offers retry", async () => {
    vi.useFakeTimers();
    vi.mocked(webBrowserMutation).mockImplementation(
      (_path, init) =>
        new Promise((_resolve, reject) => {
          init?.signal?.addEventListener("abort", () => {
            reject(new DOMException("Aborted", "AbortError"));
          });
        }),
    );

    render(<SessionRefresh />);

    await act(async () => {
      vi.advanceTimersByTime(8_000);
    });

    expect(
      screen.getByRole("button", { name: ru.workspace.sessionRetry }),
    ).toBeInTheDocument();
    expect(replace).not.toHaveBeenCalled();
  });

  it("does not duplicate the initial refresh mutation on re-render", async () => {
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
