import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { StrictMode } from "react";

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserFetch: vi.fn(),
}));

import { ru } from "@/i18n/ru";
import { getTranslator } from "@/i18n/messages";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { WorkspaceDataCacheProvider, useWorkspaceDataCache, type WorkspaceDataCache } from "@/features/workspace/WorkspaceDataCache/WorkspaceDataCache";

import { VideoFiles } from "./VideoFiles";

const firstJob = {
  id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
  status: "succeeded" as const,
  prompt: "cinematic forest walk",
  model_id: "seedance-2-5",
  model_name: "Seedance 2.5",
  duration_sec: 5,
  resolution: "1080p",
  aspect_ratio: "16:9",
  cost_estimate: 120,
  created_at: "2026-08-01T12:00:00Z",
  updated_at: "2026-08-01T12:00:00Z",
};

const secondJob = {
  ...firstJob,
  id: "4e9defcb-59d7-4d45-bc2e-7cdb770ad729",
  prompt: "rainy neon avenue",
};

const thirdJob = {
  ...firstJob,
  id: "0b2c3017-3927-427e-8b75-4204bc5a0af3",
  prompt: "macro flower opening",
};

const firstResult = {
  job_id: firstJob.id,
  status: "succeeded" as const,
  artifacts: [{
    id: "6ca96a58-a902-4f23-a92a-6726e1a0cd20",
    mime_type: "video/mp4",
    size_bytes: 1048576,
    width: 1920,
    height: 1080,
    duration_ms: 5000,
  }],
};

function listResponse(items = [firstJob], nextCursor: string | null = null) {
  return Response.json({ items, has_more: nextCursor !== null, next_cursor: nextCursor });
}

function resultResponse(jobID: string, artifactID = firstResult.artifacts[0]!.id) {
  return Response.json({
    ...firstResult,
    job_id: jobID,
    artifacts: [{ ...firstResult.artifacts[0]!, id: artifactID }],
  });
}

function CacheProbe({ onCache }: { onCache: (cache: WorkspaceDataCache) => void }) {
  onCache(useWorkspaceDataCache());
  return null;
}

describe("VideoFiles", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.resetAllMocks();
  });

  it("restores video cards and result metadata immediately after leaving and returning", async () => {
    let finish!: (response: Response) => void;
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(listResponse())
      .mockResolvedValueOnce(Response.json(firstResult))
      .mockImplementationOnce(() => new Promise<Response>(resolve => { finish = resolve; }));
    const workspace = (open: boolean) => (
      <WorkspaceDataCacheProvider>{open ? <VideoFiles visible /> : null}</WorkspaceDataCacheProvider>
    );
    const view = render(workspace(true));
    await screen.findByLabelText(`${ru.files.categories.video}: ${firstJob.prompt}`);
    view.rerender(workspace(false));
    view.rerender(workspace(true));

    const player = screen.getByLabelText(`${ru.files.categories.video}: ${firstJob.prompt}`);
    expect(document.querySelector('[data-ui="skeleton"]')).toBeNull();
    expect(webBrowserFetch).toHaveBeenCalledTimes(3);
    await act(async () => { finish(listResponse([{ ...firstJob, prompt: "updated video title" }])); });
    expect(screen.getByLabelText(`${ru.files.categories.video}: updated video title`)).toBe(player);
    expect(webBrowserFetch).toHaveBeenCalledTimes(3);
  });

  it("keeps cached video cards after a failed background refresh and offers a retry", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(listResponse())
      .mockResolvedValueOnce(Response.json(firstResult))
      .mockResolvedValueOnce(new Response(null, { status: 503 }))
      .mockResolvedValueOnce(listResponse([secondJob]))
      .mockResolvedValueOnce(resultResponse(secondJob.id));
    const workspace = (open: boolean) => (
      <WorkspaceDataCacheProvider>{open ? <VideoFiles visible /> : null}</WorkspaceDataCacheProvider>
    );
    const view = render(workspace(true));
    await screen.findByLabelText(`${ru.files.categories.video}: ${firstJob.prompt}`);
    view.rerender(workspace(false));
    view.rerender(workspace(true));

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.files.loadFailure);
    expect(screen.getByLabelText(`${ru.files.categories.video}: ${firstJob.prompt}`)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: ru.files.retry }));
    expect(await screen.findByLabelText(`${ru.files.categories.video}: ${secondJob.prompt}`)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("does not replace a cached empty video page with skeletons on return", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(listResponse([]))
      .mockReturnValueOnce(new Promise<Response>(() => {}));
    const workspace = (open: boolean) => (
      <WorkspaceDataCacheProvider>{open ? <VideoFiles visible /> : null}</WorkspaceDataCacheProvider>
    );
    const view = render(workspace(true));
    await screen.findByText(ru.files.emptyVideoDescription);
    view.rerender(workspace(false));
    view.rerender(workspace(true));

    expect(screen.getByText(ru.files.emptyVideoDescription)).toBeInTheDocument();
    expect(document.querySelector('[data-ui="skeleton"]')).toBeNull();
    expect(webBrowserFetch).toHaveBeenCalledTimes(2);
  });

  it("aborts the discarded StrictMode read and prevents its late result from overwriting the cache", async () => {
    let finishOld!: (response: Response) => void;
    let cache!: WorkspaceDataCache;
    vi.mocked(webBrowserFetch)
      .mockImplementationOnce(() => new Promise<Response>(resolve => { finishOld = resolve; }))
      .mockResolvedValueOnce(listResponse([secondJob]))
      .mockResolvedValueOnce(resultResponse(secondJob.id));
    render(
      <StrictMode>
        <WorkspaceDataCacheProvider>
          <CacheProbe onCache={value => { cache = value; }} />
          <VideoFiles visible />
        </WorkspaceDataCacheProvider>
      </StrictMode>,
    );
    await screen.findByLabelText(`${ru.files.categories.video}: ${secondJob.prompt}`);
    const discardedSignal = vi.mocked(webBrowserFetch).mock.calls[0]![1]?.signal;
    expect(discardedSignal?.aborted).toBe(true);
    await act(async () => { finishOld(listResponse([firstJob])); });
    expect(screen.queryByText(firstJob.prompt)).not.toBeInTheDocument();
    expect(cache.getVideoFilesFirstPage()?.items).toEqual([secondJob]);
  });

  it("clears cached private videos on an account boundary and ignores the old account refresh", async () => {
    let finishOld!: (response: Response) => void;
    let finishNew!: (response: Response) => void;
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(listResponse())
      .mockResolvedValueOnce(Response.json(firstResult))
      .mockImplementationOnce(() => new Promise<Response>(resolve => { finishOld = resolve; }))
      .mockImplementationOnce(() => new Promise<Response>(resolve => { finishNew = resolve; }))
      .mockResolvedValueOnce(resultResponse(secondJob.id));
    const workspace = (accountId: string, open: boolean) => (
      <WorkspaceDataCacheProvider key={accountId}>{open ? <VideoFiles visible /> : null}</WorkspaceDataCacheProvider>
    );
    const view = render(workspace("account-a", true));
    await screen.findByLabelText(`${ru.files.categories.video}: ${firstJob.prompt}`);
    view.rerender(workspace("account-a", false));
    view.rerender(workspace("account-a", true));
    expect(screen.getByText(firstJob.prompt)).toBeInTheDocument();
    view.rerender(workspace("account-b", true));
    expect(screen.queryByText(firstJob.prompt)).not.toBeInTheDocument();
    expect(vi.mocked(webBrowserFetch).mock.calls[2]![1]?.signal?.aborted).toBe(true);
    await act(async () => { finishOld(listResponse([thirdJob])); });
    expect(screen.queryByText(thirdJob.prompt)).not.toBeInTheDocument();
    await act(async () => { finishNew(listResponse([secondJob])); });
    expect(await screen.findByLabelText(`${ru.files.categories.video}: ${secondJob.prompt}`)).toBeInTheDocument();
  });

  it("does not fetch while hidden and keeps loaded videos across visibility switches", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(listResponse())
      .mockResolvedValueOnce(Response.json(firstResult));
    const view = render(<VideoFiles visible={false} />);

    expect(webBrowserFetch).not.toHaveBeenCalled();
    view.rerender(<VideoFiles visible />);

    const video = await screen.findByLabelText(`${ru.files.categories.video}: ${firstJob.prompt}`);
    expect(webBrowserFetch).toHaveBeenNthCalledWith(1, "/web/v1/video-jobs?limit=12", expect.objectContaining({ signal: expect.any(AbortSignal) }));
    expect(webBrowserFetch).toHaveBeenNthCalledWith(2, `/web/v1/video-jobs/${firstJob.id}/result`, expect.objectContaining({ signal: expect.any(AbortSignal) }));
    expect(video).toHaveAttribute("controls");
    expect(video).toHaveAttribute("src", `/web/v1/video-artifacts/${firstResult.artifacts[0]!.id}`);
    expect(screen.getByRole("link", { name: `${ru.files.download}: ${firstJob.prompt}` })).toHaveAttribute("href", `/web/v1/video-artifacts/${firstResult.artifacts[0]!.id}`);
    expect(screen.getByRole("heading", { name: firstJob.prompt })).toBeInTheDocument();
    expect(screen.getByText(`Seedance 2.5 · 1080p · ${getTranslator("ru")("generationOptions.valueS", { value1: 5 })} · 16:9`)).toBeInTheDocument();

    view.rerender(<VideoFiles visible={false} />);
    expect(screen.queryByLabelText(`${ru.files.categories.video}: ${firstJob.prompt}`)).not.toBeInTheDocument();
    view.rerender(<VideoFiles visible />);
    expect(await screen.findByLabelText(`${ru.files.categories.video}: ${firstJob.prompt}`)).toBeInTheDocument();
    expect(webBrowserFetch).toHaveBeenCalledTimes(2);
  });

  it("loads additional pages without replacing already loaded videos", async () => {
    let firstPageRequests = 0;
    vi.mocked(webBrowserFetch).mockImplementation((path) => {
      if (path === "/web/v1/video-jobs?limit=12") {
        firstPageRequests += 1;
        return Promise.resolve(listResponse([firstJob], "next-page"));
      }
      if (path === "/web/v1/video-jobs?limit=12&cursor=next-page") {
        return Promise.resolve(listResponse([secondJob], null));
      }
      if (path === `/web/v1/video-jobs/${secondJob.id}/result`) {
        return Promise.resolve(resultResponse(secondJob.id, "d066a4a0-647e-407b-b9c4-2c5b03a4b0da"));
      }
      if (path === `/web/v1/video-jobs/${firstJob.id}/result`) {
        return Promise.resolve(resultResponse(firstJob.id));
      }
      return Promise.resolve(new Response(null, { status: 404 }));
    });

    render(<VideoFiles visible />);
    await screen.findByText(firstJob.prompt);
    fireEvent.click(screen.getByRole("button", { name: ru.files.loadMore }));
    expect(await screen.findByText(secondJob.prompt)).toBeInTheDocument();
    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/video-jobs?limit=12&cursor=next-page", expect.objectContaining({ signal: expect.any(AbortSignal) }));

    expect(screen.getByText(firstJob.prompt)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: ru.files.loadMore })).not.toBeInTheDocument();
    expect(firstPageRequests).toBe(1);
  });

  it("retries list and result failures while honoring hidden empty state", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(new Response(null, { status: 503 }))
      .mockResolvedValueOnce(listResponse([firstJob], null))
      .mockResolvedValueOnce(new Response(null, { status: 404 }))
      .mockResolvedValueOnce(Response.json(firstResult));

    render(<VideoFiles showEmpty={false} visible />);
    expect(await screen.findByRole("alert")).toHaveTextContent(ru.files.loadFailure);
    expect(screen.queryByText(ru.files.emptyVideoDescription)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: ru.files.retry }));
    const card = await screen.findByRole("article", { name: firstJob.prompt });
    const retry = await within(card).findByRole("button", { name: ru.files.previewRetry });
    fireEvent.click(retry);
    expect(await screen.findByLabelText(`${ru.files.categories.video}: ${firstJob.prompt}`)).toBeInTheDocument();
  });

  it("caps concurrent result fetches at two", async () => {
    const pendingResults: Array<() => void> = [];
    vi.mocked(webBrowserFetch).mockImplementation((path) => {
      if (path === "/web/v1/video-jobs?limit=12") {
        return Promise.resolve(listResponse([firstJob, secondJob, thirdJob], null));
      }
      return new Promise<Response>((resolve) => {
        pendingResults.push(() => resolve(Response.json({
          ...firstResult,
          job_id: String(path).includes(secondJob.id) ? secondJob.id : String(path).includes(thirdJob.id) ? thirdJob.id : firstJob.id,
          artifacts: [{ ...firstResult.artifacts[0]!, id: `00000000-0000-4000-8000-${String(pendingResults.length).padStart(12, "0")}` }],
        })));
      });
    });

    render(<VideoFiles visible />);
    await screen.findByText(firstJob.prompt);
    await waitFor(() => expect(vi.mocked(webBrowserFetch).mock.calls.filter(([path]) => String(path).includes("/result"))).toHaveLength(2));
    expect(pendingResults).toHaveLength(2);
    pendingResults[0]!();
    await waitFor(() => expect(vi.mocked(webBrowserFetch).mock.calls.filter(([path]) => String(path).includes("/result"))).toHaveLength(3));
  });
});
