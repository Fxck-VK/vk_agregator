import { useState } from "react";

import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserFetch: vi.fn(),
}));

import { ru } from "@/i18n/ru";
import type { ImageJob } from "@/lib/web-api/contracts";
import { webBrowserFetch } from "@/lib/web-api/browser";

import { ImageJobTracker } from "./ImageJobTracker";

const queuedJob = {
  id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
  status: "queued" as const,
  prompt: "night city after rain",
  model_id: "nano-banana-2",
  model_name: "Nano Banana 2",
  image_quality: "2K",
  cost_estimate: 60,
  created_at: "2026-08-01T12:00:00Z",
  updated_at: "2026-08-01T12:00:00Z",
};

describe("ImageJobTracker", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.resetAllMocks();
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it("polls exactly the visible active job after the first bounded delay", async () => {
    const onJobUpdate = vi.fn();
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ job: queuedJob }));

    render(
      <ImageJobTracker
        job={queuedJob}
        onError={vi.fn()}
        onJobUpdate={onJobUpdate}
        onResult={vi.fn()}
      />,
    );

    expect(webBrowserFetch).not.toHaveBeenCalled();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(webBrowserFetch).toHaveBeenCalledTimes(1);
    expect(webBrowserFetch).toHaveBeenCalledWith(`/web/v1/image-jobs/${queuedJob.id}`);
    expect(onJobUpdate).toHaveBeenCalledWith(queuedJob);
    expect(screen.getByRole("heading", { name: ru.imageGeneration.statusTitle })).toBeInTheDocument();
  });

  it("loads the result and stops polling after a succeeded job", async () => {
    const onResult = vi.fn();
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ job: { ...queuedJob, status: "succeeded" } }))
      .mockResolvedValueOnce(
        Response.json({
          job_id: queuedJob.id,
          status: "succeeded",
          artifacts: [
            {
              id: "4e9defcb-59d7-4d45-bc2e-7cdb770ad729",
              mime_type: "image/png",
              size_bytes: 42,
              width: 1024,
              height: 1024,
            },
          ],
        }),
      );

    render(
      <ImageJobTracker
        job={queuedJob}
        onError={vi.fn()}
        onJobUpdate={vi.fn()}
        onResult={onResult}
      />,
    );

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30000);
    });

    expect(webBrowserFetch).toHaveBeenCalledTimes(2);
    expect(onResult).toHaveBeenCalledWith(expect.objectContaining({ job_id: queuedJob.id }));
  });

  it("recovers the result when activation is replayed with an already succeeded job", async () => {
    const onResult = vi.fn();
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ job: { ...queuedJob, status: "succeeded" } }))
      .mockResolvedValueOnce(
        Response.json({
          job_id: queuedJob.id,
          status: "succeeded",
          artifacts: [
            {
              id: "6f8736b8-7b33-48e8-8bdc-849dbda55d3c",
              mime_type: "image/png",
              size_bytes: 42,
              width: 1024,
              height: 1024,
            },
          ],
        }),
      );

    render(
      <ImageJobTracker
        job={{ ...queuedJob, status: "succeeded" }}
        onError={vi.fn()}
        onJobUpdate={vi.fn()}
        onResult={onResult}
      />,
    );

    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(webBrowserFetch).toHaveBeenNthCalledWith(1, `/web/v1/image-jobs/${queuedJob.id}`);
    expect(webBrowserFetch).toHaveBeenNthCalledWith(2, `/web/v1/image-jobs/${queuedJob.id}/result`);
    expect(onResult).toHaveBeenCalledWith(expect.objectContaining({ job_id: queuedJob.id }));
  });

  it("keeps increasing the delay when a visible job refreshes with the same status", async () => {
    const onError = vi.fn();
    const onResult = vi.fn();
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ job: { ...queuedJob } }));

    function RerenderingTracker() {
      const [job, setJob] = useState<ImageJob>(queuedJob);
      return <ImageJobTracker job={job} onError={onError} onJobUpdate={setJob} onResult={onResult} />;
    }

    render(<RerenderingTracker />);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(webBrowserFetch).toHaveBeenCalledTimes(1);
  });

  it("does not schedule status traffic while the browser tab is hidden", async () => {
    Object.defineProperty(document, "visibilityState", { configurable: true, value: "hidden" });
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ job: queuedJob }));

    render(
      <ImageJobTracker
        job={queuedJob}
        onError={vi.fn()}
        onJobUpdate={vi.fn()}
        onResult={vi.fn()}
      />,
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30000);
    });

    expect(webBrowserFetch).not.toHaveBeenCalled();
    Object.defineProperty(document, "visibilityState", { configurable: true, value: "visible" });
  });
});
