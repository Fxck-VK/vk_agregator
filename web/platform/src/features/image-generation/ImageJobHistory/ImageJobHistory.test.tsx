import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserFetch: vi.fn(),
}));

import { ru } from "@/i18n/ru";
import { webBrowserFetch } from "@/lib/web-api/browser";

import { ImageJobHistory } from "./ImageJobHistory";

const succeededJob = {
  id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
  status: "succeeded",
  prompt: "night city after rain",
  model_id: "nano-banana-2",
  model_name: "Nano Banana 2",
  image_quality: "2K",
  cost_estimate: 60,
  created_at: "2026-08-01T12:00:00Z",
  updated_at: "2026-08-01T12:00:00Z",
};

describe("ImageJobHistory", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("loads a bounded history only after the user requests it", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(
      Response.json({
        items: [succeededJob],
        has_more: true,
        next_cursor: "opaque-next-page-cursor",
      }),
    );
    render(<ImageJobHistory />);

    expect(webBrowserFetch).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: ru.imageHistory.load }));

    expect(await screen.findByText(succeededJob.prompt)).toBeInTheDocument();
    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/image-jobs?limit=10");
    expect(screen.getByRole("button", { name: ru.imageHistory.loadMore })).toBeInTheDocument();
  });

  it("uses the opaque cursor for the next page", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(
        Response.json({
          items: [succeededJob],
          has_more: true,
          next_cursor: "opaque-next-page-cursor",
        }),
      )
      .mockResolvedValueOnce(
        Response.json({
          items: [{ ...succeededJob, id: "4e9defcb-59d7-4d45-bc2e-7cdb770ad729", prompt: "older image" }],
          has_more: false,
          next_cursor: null,
        }),
      );
    render(<ImageJobHistory />);

    fireEvent.click(screen.getByRole("button", { name: ru.imageHistory.load }));
    await screen.findByText(succeededJob.prompt);
    fireEvent.click(screen.getByRole("button", { name: ru.imageHistory.loadMore }));

    expect(await screen.findByText("older image")).toBeInTheDocument();
    expect(webBrowserFetch).toHaveBeenLastCalledWith("/web/v1/image-jobs?limit=10&cursor=opaque-next-page-cursor");
  });

  it("opens and manually refreshes a successful result only through the platform artifact path", async () => {
    const artifactID = "4e9defcb-59d7-4d45-bc2e-7cdb770ad729";
    const result = {
      job_id: succeededJob.id,
      status: "succeeded",
      artifacts: [
        {
          id: artifactID,
          mime_type: "image/png",
          size_bytes: 42,
          width: 1024,
          height: 1024,
        },
      ],
    };
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [succeededJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json(result))
      .mockResolvedValueOnce(Response.json(result));
    render(<ImageJobHistory />);

    fireEvent.click(screen.getByRole("button", { name: ru.imageHistory.load }));
    await screen.findByText(succeededJob.prompt);
    fireEvent.click(screen.getByRole("button", { name: ru.imageHistory.openResult }));

    const image = await screen.findByRole("img", { name: ru.imageHistory.resultImageAlt });
    expect(image).toHaveAttribute("src", `/web/v1/image-artifacts/${artifactID}`);
    expect(screen.queryByText("https://objects.example.test/private-key")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: ru.imageHistory.refreshResult }));
    await vi.waitFor(() =>
      expect(webBrowserFetch).toHaveBeenLastCalledWith(`/web/v1/image-jobs/${succeededJob.id}/result`),
    );
  });
});
