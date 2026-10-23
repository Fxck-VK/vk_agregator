import { useLayoutEffect, useState } from "react";

import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
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

const completedResult = {
  job_id: queuedJob.id, status: "succeeded",
  artifacts: [{ id: "4e9defcb-59d7-4d45-bc2e-7cdb770ad729", mime_type: "image/png", size_bytes: 42, width: 1024, height: 1024 }],
};

describe("ImageJobTracker", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.resetAllMocks();
  });

  afterEach(() => {
    cleanup();
    Object.defineProperty(navigator, "onLine", { configurable: true, value: true });
    vi.useRealTimers();
  });

  it("handles a manual refresh as soon as the status control is committed", async () => {
    const onError = vi.fn();
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ job: { ...queuedJob, status: "succeeded" } }))
      .mockResolvedValueOnce(Response.json({ job_id: queuedJob.id, status: "succeeded", artifacts: [] }));

    function ImmediateRefresh() {
      useLayoutEffect(() => {
        screen.getByRole("button", { name: ru.imageGeneration.statusRefresh }).click();
      }, []);
      return <ImageJobTracker job={queuedJob} onError={onError} onJobUpdate={vi.fn()} onResult={vi.fn()} />;
    }

    render(<ImmediateRefresh />);
    await act(async () => { await vi.advanceTimersByTimeAsync(0); });

    expect(webBrowserFetch).toHaveBeenCalledTimes(2);
    expect(onError).toHaveBeenCalledWith("result");
    expect(screen.getByRole("alert")).toHaveTextContent(ru.imageGeneration.resultFailure);
    expect(screen.getByRole("button", { name: ru.imageGeneration.statusRefresh })).toBeEnabled();
  });

  it("keeps a pending job offline and retrieves its result immediately after reconnect", async () => {
    Object.defineProperty(navigator, "onLine", { configurable: true, value: false });
    const onResult = vi.fn();
    const onError = vi.fn();
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ job: { ...queuedJob, status: "succeeded" } }))
      .mockResolvedValueOnce(Response.json(completedResult));
    const { unmount } = render(<ImageJobTracker job={queuedJob} onError={onError} onJobUpdate={vi.fn()} onResult={onResult} />);
    await act(async () => { await vi.advanceTimersByTimeAsync(60000); });
    expect(webBrowserFetch).not.toHaveBeenCalled();
    expect(screen.getByText(ru.imageGeneration.statusQueued)).toBeVisible();
    await act(async () => {
      Object.defineProperty(navigator, "onLine", { configurable: true, value: true });
      window.dispatchEvent(new Event("online"));
      window.dispatchEvent(new Event("online"));
    });
    expect(webBrowserFetch).toHaveBeenCalledTimes(2);
    expect(onResult).toHaveBeenCalledOnce();
    expect(onError).not.toHaveBeenCalled();
    unmount();
    window.dispatchEvent(new Event("online"));
    expect(webBrowserFetch).toHaveBeenCalledTimes(2);
  });

  it("aborts an interrupted result read and retries without publishing a stale result", async () => {
    let finish!: (response: Response) => void;
    const onError = vi.fn();
    const onResult = vi.fn();
    const onJobUpdate = vi.fn();
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ job: { ...queuedJob, status: "succeeded" } }))
      .mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }))
      .mockResolvedValueOnce(Response.json({ job: { ...queuedJob, status: "succeeded" } }))
      .mockResolvedValueOnce(Response.json(completedResult));
    render(<ImageJobTracker job={queuedJob} onError={onError} onJobUpdate={onJobUpdate} onResult={onResult} />);
    await act(async () => { await vi.advanceTimersByTimeAsync(1500); });
    const signal = vi.mocked(webBrowserFetch).mock.calls[1][1]?.signal;
    await act(async () => {
      Object.defineProperty(navigator, "onLine", { configurable: true, value: false });
      window.dispatchEvent(new Event("offline"));
    });
    expect(signal?.aborted).toBe(true);
    expect(onJobUpdate).not.toHaveBeenCalled();
    await act(async () => {
      Object.defineProperty(navigator, "onLine", { configurable: true, value: true });
      window.dispatchEvent(new Event("online"));
    });
    expect(onResult).toHaveBeenCalledOnce();
    await act(async () => { finish(Response.json(completedResult)); });
    expect(onResult).toHaveBeenCalledOnce();
    expect(onError).not.toHaveBeenCalled();
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
    expect(webBrowserFetch).toHaveBeenCalledWith(`/web/v1/image-jobs/${queuedJob.id}`, expect.objectContaining({ signal: expect.any(AbortSignal) }));
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

  it("preserves manual retry for an already completed job after a genuine status error", async () => {
    const onResult = vi.fn();
    vi.mocked(webBrowserFetch).mockRejectedValue(new Error("unavailable"));
    render(<ImageJobTracker job={{ ...queuedJob, status: "succeeded" }} onJobUpdate={vi.fn()} onResult={onResult} />);
    await act(async () => { await vi.advanceTimersByTimeAsync(60000); });
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ job: { ...queuedJob, status: "succeeded" } }))
      .mockResolvedValueOnce(Response.json(completedResult));
    await act(async () => { fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.statusRefresh })); });
    expect(onResult).toHaveBeenCalledOnce();
    expect(webBrowserFetch).toHaveBeenCalledTimes(3);
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

    expect(webBrowserFetch).toHaveBeenNthCalledWith(1, `/web/v1/image-jobs/${queuedJob.id}`, expect.objectContaining({ signal: expect.any(AbortSignal) }));
    expect(webBrowserFetch).toHaveBeenNthCalledWith(2, `/web/v1/image-jobs/${queuedJob.id}/result`, expect.objectContaining({ signal: expect.any(AbortSignal) }));
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
