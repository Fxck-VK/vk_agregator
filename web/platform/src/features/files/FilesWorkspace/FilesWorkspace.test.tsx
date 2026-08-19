import { StrictMode } from "react";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserFetch: vi.fn(),
  webBrowserMutation: vi.fn(),
}));

import { ru } from "@/i18n/ru";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import type { ImageJob, ImageJobList } from "@/lib/web-api/contracts";
import {
  useWorkspaceDataCache,
  type WorkspaceDataCache,
  WorkspaceDataCacheProvider,
} from "@/features/workspace/WorkspaceDataCache/WorkspaceDataCache";

import { FilesWorkspace } from "./FilesWorkspace";

function WorkspaceDataCacheSeed({
  page,
  retryReplacement,
  onCache,
}: {
  onCache?: (cache: WorkspaceDataCache) => void;
  page?: ImageJobList;
  retryReplacement?: { job: ImageJob; originalJobID: string };
}) {
  const cache = useWorkspaceDataCache();

  onCache?.(cache);

  if (page !== undefined) {
    cache.setImageFilesFirstPage(page);
  }
  if (retryReplacement !== undefined) {
    cache.setImageFileRetryReplacement(retryReplacement);
  }

  return null;
}

function renderFilesWorkspace({
  cachePage,
  initialCategory,
  onCache,
  retryReplacement,
  strictMode = false,
}: {
  cachePage?: ImageJobList;
  initialCategory?: "all" | "images" | "reports" | "presentations" | "video" | "uploads";
  onCache?: (cache: WorkspaceDataCache) => void;
  retryReplacement?: { job: ImageJob; originalJobID: string };
  strictMode?: boolean;
} = {}) {
  const workspace = <FilesWorkspace initialCategory={initialCategory} />;

  return render(
    <WorkspaceDataCacheProvider>
      <WorkspaceDataCacheSeed onCache={onCache} page={cachePage} retryReplacement={retryReplacement} />
      {strictMode ? <StrictMode>{workspace}</StrictMode> : workspace}
    </WorkspaceDataCacheProvider>,
  );
}

const firstSucceededJob = {
  id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
  status: "succeeded" as const,
  prompt: "night city after rain",
  model_id: "nano-banana-2",
  model_name: "Nano Banana 2",
  image_quality: "2K",
  cost_estimate: 60,
  created_at: "2026-08-01T12:00:00Z",
  updated_at: "2026-08-01T12:00:00Z",
};

const secondSucceededJob = {
  ...firstSucceededJob,
  id: "4e9defcb-59d7-4d45-bc2e-7cdb770ad729",
  prompt: "sunlit mountain lake",
};

const thirdSucceededJob = {
  ...firstSucceededJob,
  id: "0b2c3017-3927-427e-8b75-4204bc5a0af3",
  prompt: "misty morning forest",
};

const firstResult = {
  job_id: firstSucceededJob.id,
  status: "succeeded" as const,
  artifacts: [
    {
      id: "6ca96a58-a902-4f23-a92a-6726e1a0cd20",
      mime_type: "image/png",
      size_bytes: 42,
      width: 1024,
      height: 1024,
    },
  ],
};

const secondResult = {
  job_id: secondSucceededJob.id,
  status: "succeeded" as const,
  artifacts: [
    {
      id: "d066a4a0-647e-407b-b9c4-2c5b03a4b0da",
      mime_type: "image/png",
      size_bytes: 84,
      width: 1024,
      height: 1024,
    },
  ],
};

const expiredPreparationJob = {
  ...firstSucceededJob,
  id: "6db2f5ed-7b3f-4e32-9a3e-b6e50d2d2a4d",
  status: "expired" as const,
};

const retriedQueuedJob = {
  ...expiredPreparationJob,
  id: "e585d540-a579-4f59-b6cd-f95288be4c14",
  status: "queued" as const,
  created_at: "2026-08-03T12:00:00Z",
  updated_at: "2026-08-03T12:00:00Z",
};

const retriedResult = {
  job_id: retriedQueuedJob.id,
  status: "succeeded" as const,
  artifacts: [
    {
      id: "20993a1e-ef04-4d76-a751-f553401b5cbd",
      mime_type: "image/png",
      size_bytes: 96,
      width: 1024,
      height: 1024,
    },
  ],
};

describe("FilesWorkspace", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.resetAllMocks();
  });

  it("opens the file category requested by the route", () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(
      Response.json({ items: [], has_more: false, next_cursor: null }),
    );

    renderFilesWorkspace({ initialCategory: "uploads" });

    expect(screen.getByRole("tab", { name: ru.files.categories.uploads })).toHaveAttribute("aria-selected", "true");
  });

  it("loads a bounded generated-image page and previews artifacts only through the platform path", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [firstSucceededJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json(firstResult));

    renderFilesWorkspace();

    const image = await screen.findByRole("img", { name: ru.files.generatedImageAlt });
    expect(webBrowserFetch).toHaveBeenNthCalledWith(1, "/web/v1/image-jobs?limit=12");
    expect(webBrowserFetch).toHaveBeenNthCalledWith(2, `/web/v1/image-jobs/${firstSucceededJob.id}/result`);
    expect(image).toHaveAttribute("src", "/web/v1/image-artifacts/6ca96a58-a902-4f23-a92a-6726e1a0cd20");
    expect(screen.queryByText("https://objects.example.test/private-key")).not.toBeInTheDocument();
  });

  it("keeps image previews available in React Strict Mode", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [firstSucceededJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json(firstResult));

    renderFilesWorkspace({ strictMode: true });

    expect(await screen.findByRole("img", { name: ru.files.generatedImageAlt })).toHaveAttribute(
      "src",
      "/web/v1/image-artifacts/6ca96a58-a902-4f23-a92a-6726e1a0cd20",
    );
  });

  it("filters only the currently loaded result cards without another list request", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [firstSucceededJob, secondSucceededJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json(firstResult))
      .mockResolvedValueOnce(Response.json(secondResult));

    renderFilesWorkspace();

    await screen.findByText(firstSucceededJob.prompt);
    await screen.findByText(secondSucceededJob.prompt);
    fireEvent.change(screen.getByRole("searchbox", { name: ru.files.searchLabel }), { target: { value: "mountain" } });

    expect(screen.queryByText(firstSucceededJob.prompt)).not.toBeInTheDocument();
    expect(screen.getByText(secondSucceededJob.prompt)).toBeInTheDocument();
    expect(webBrowserFetch).toHaveBeenCalledTimes(3);
  });

  it("filters loaded cards by ready and in-progress state", async () => {
    const pendingJob = { ...secondSucceededJob, status: "queued" as const, prompt: "queued forest image" };
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [firstSucceededJob, pendingJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json(firstResult));

    renderFilesWorkspace();

    await screen.findByText(firstSucceededJob.prompt);
    await screen.findByText(pendingJob.prompt);
    fireEvent.change(screen.getByRole("combobox", { name: ru.files.statusFilterLabel }), { target: { value: "ready" } });

    expect(screen.getByText(firstSucceededJob.prompt)).toBeInTheDocument();
    expect(screen.queryByText(pendingJob.prompt)).not.toBeInTheDocument();

    fireEvent.change(screen.getByRole("combobox", { name: ru.files.statusFilterLabel }), { target: { value: "in_progress" } });
    expect(screen.queryByText(firstSucceededJob.prompt)).not.toBeInTheDocument();
    expect(screen.getByText(pendingJob.prompt)).toBeInTheDocument();
  });

  it("keeps image files available while future categories show their own empty state without another request", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [firstSucceededJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json(firstResult));

    renderFilesWorkspace();

    expect(await screen.findByText(firstSucceededJob.prompt)).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Все файлы" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getAllByRole("tab").map((tab) => tab.textContent)).toEqual([
      "Все файлы",
      "Изображения",
      "Рефераты",
      "Презентации",
      "Видео",
      "Загруженные",
    ]);

    const requestCountBeforeTabSwitch = vi.mocked(webBrowserFetch).mock.calls.length;
    fireEvent.click(screen.getByRole("tab", { name: "Рефераты" }));

    expect(screen.getByRole("tab", { name: "Рефераты" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("heading", { name: "Пока ничего нет" })).toBeInTheDocument();
    expect(screen.getByText("Здесь будут ваши рефераты.")).toBeInTheDocument();
    expect(screen.queryByText(firstSucceededJob.prompt)).not.toBeInTheDocument();
    expect(webBrowserFetch).toHaveBeenCalledTimes(requestCountBeforeTabSwitch);

    fireEvent.click(screen.getByRole("tab", { name: "Изображения" }));

    expect(screen.getByRole("tab", { name: "Изображения" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText(firstSucceededJob.prompt)).toBeInTheDocument();

    fireEvent.keyDown(screen.getByRole("tab", { name: "Изображения" }), { key: "ArrowRight" });

    expect(screen.getByRole("tab", { name: "Рефераты" })).toHaveAttribute("aria-selected", "true");
  });

  it("uses the illustrated library empty state when there are no image files", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json({ items: [], has_more: false, next_cursor: null }));

    renderFilesWorkspace();

    expect(await screen.findByRole("heading", { name: "Пока ничего нет" })).toBeInTheDocument();
    expect(screen.getByText("Здесь будут храниться ваши сгенерированные изображения и другие файлы.")).toBeInTheDocument();
    expect(screen.queryByRole("searchbox", { name: ru.files.searchLabel })).not.toBeInTheDocument();
  });

  it("explains jobs that stopped before submission and restores an expired request for retry", async () => {
    const awaitingPaymentJob = {
      ...firstSucceededJob,
      id: "c90a04c6-8f0c-4c94-bfe2-b3ca0b72f2ec",
      status: "awaiting_payment" as const,
      prompt: "bmx monkey",
      model_id: "gpt-image-2",
      model_name: "GPT Image 2",
      image_quality: "1K",
    };
    const expiredPreparationJob = {
      ...awaitingPaymentJob,
      id: "6db2f5ed-7b3f-4e32-9a3e-b6e50d2d2a4d",
      status: "expired" as const,
      prompt: "night city after rain",
    };
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json({
      items: [awaitingPaymentJob, expiredPreparationJob],
      has_more: false,
      next_cursor: null,
    }));

    renderFilesWorkspace();

    expect(await screen.findByText("Не хватило токенов")).toBeInTheDocument();
    expect(screen.getByText("Пополните баланс, чтобы повторить запуск.")).toBeInTheDocument();
    expect(screen.getByText("Запрос не был отправлен")).toBeInTheDocument();
    expect(screen.getByText("Подтверждение запуска истекло.")).toBeInTheDocument();
    const retryButtons = screen.getAllByRole("button", { name: "Повторить" });
    expect(retryButtons).toHaveLength(2);
    retryButtons.forEach((button) => expect(button).toBeEnabled());
    expect(screen.queryByRole("link", { name: "Повторить" })).not.toBeInTheDocument();
    expect(screen.queryAllByText(ru.files.noReadyArtifact)).toHaveLength(0);
  });

  it("activates an awaiting-payment image in place without leaving the files workspace", async () => {
    const awaitingPaymentJob = {
      ...firstSucceededJob,
      id: "a71ccdbd-4d92-4942-b745-8f6690c5576a",
      status: "awaiting_payment" as const,
      prompt: "awaiting payment image",
    };
    const activatedQueuedJob = {
      ...awaitingPaymentJob,
      status: "queued" as const,
      updated_at: "2026-08-03T13:00:00Z",
    };
    const activatedSucceededJob = {
      ...activatedQueuedJob,
      status: "succeeded" as const,
      updated_at: "2026-08-03T13:00:01Z",
    };
    const activatedResult = {
      ...firstResult,
      job_id: awaitingPaymentJob.id,
    };
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [awaitingPaymentJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json({ job: activatedSucceededJob }))
      .mockResolvedValueOnce(Response.json(activatedResult));
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json({ job: activatedQueuedJob }, { status: 200 }));
    window.history.replaceState(null, "", "/app/files");

    renderFilesWorkspace();

    const card = (await screen.findByText(awaitingPaymentJob.prompt)).closest("article");
    expect(card).not.toBeNull();
    fireEvent.click(within(card!).getByRole("button", { name: ru.files.retry }));

    expect(window.location.pathname).toBe("/app/files");
    expect(webBrowserMutation).toHaveBeenCalledWith(`/web/v1/image-jobs/${awaitingPaymentJob.id}/activate`, {
      method: "POST",
    });
    expect(card).toHaveAttribute("aria-busy", "true");
    expect(within(card!).getByRole("status", { name: ru.files.retrying })).toBeInTheDocument();

    expect(await screen.findByRole("img", { name: ru.files.generatedImageAlt }, { timeout: 3000 })).toBeInTheDocument();
    expect(webBrowserFetch).toHaveBeenNthCalledWith(2, `/web/v1/image-jobs/${awaitingPaymentJob.id}`, expect.anything());
    expect(webBrowserFetch).toHaveBeenNthCalledWith(3, `/web/v1/image-jobs/${awaitingPaymentJob.id}/result`);
  });

  it("keeps an awaiting-payment image retryable when its balance is still insufficient", async () => {
    const awaitingPaymentJob = {
      ...firstSucceededJob,
      id: "d9bdece2-c61b-4bde-a202-a20e8b2c9d51",
      status: "awaiting_payment" as const,
      prompt: "still awaiting payment image",
    };
    const stillAwaitingPaymentJob = {
      ...awaitingPaymentJob,
      updated_at: "2026-08-03T13:00:01Z",
    };
    let settleActivation: (response: Response) => void = () => {};
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json({
      items: [awaitingPaymentJob],
      has_more: false,
      next_cursor: null,
    }));
    vi.mocked(webBrowserMutation).mockReturnValueOnce(new Promise<Response>((resolve) => {
      settleActivation = resolve;
    }));
    window.history.replaceState(null, "", "/app/files");

    renderFilesWorkspace();

    const card = (await screen.findByText(awaitingPaymentJob.prompt)).closest("article");
    expect(card).not.toBeNull();
    fireEvent.click(within(card!).getByRole("button", { name: ru.files.retry }));

    expect(card).toHaveAttribute("aria-busy", "true");
    expect(within(card!).getByRole("status", { name: ru.files.retrying })).toBeInTheDocument();
    settleActivation(Response.json({ job: stillAwaitingPaymentJob }, { status: 402 }));

    expect(await within(card!).findByRole("button", { name: ru.files.retry })).toBeEnabled();
    expect(within(card!).getByText(ru.files.insufficientTokensDescription)).toBeInTheDocument();
    expect(window.location.pathname).toBe("/app/files");
    expect(webBrowserMutation).toHaveBeenCalledWith(`/web/v1/image-jobs/${awaitingPaymentJob.id}/activate`, {
      method: "POST",
    });
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);
  });

  it("keeps an activated image ahead of a stale cached-page revalidation", async () => {
    const awaitingPaymentJob = {
      ...firstSucceededJob,
      id: "f49bb0a7-17d1-4d89-9d85-7d0738bfd190",
      status: "awaiting_payment" as const,
      prompt: "cached payment image",
      updated_at: "2026-08-03T13:00:00Z",
    };
    const activatedQueuedJob = {
      ...awaitingPaymentJob,
      status: "queued" as const,
      updated_at: "2026-08-03T13:00:01Z",
    };
    const cachedPage: ImageJobList = {
      items: [awaitingPaymentJob],
      has_more: false,
      next_cursor: null,
    };
    let observedCache: WorkspaceDataCache | undefined;
    let resolveRevalidation: (response: Response) => void = () => {};
    const revalidation = new Promise<Response>((resolve) => {
      resolveRevalidation = resolve;
    });
    vi.mocked(webBrowserFetch).mockReturnValueOnce(revalidation);
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json({ job: activatedQueuedJob }, { status: 200 }));

    renderFilesWorkspace({ cachePage: cachedPage, onCache: (cache) => { observedCache = cache; } });

    await vi.waitFor(() => expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/image-jobs?limit=12"));
    const card = screen.getByText(awaitingPaymentJob.prompt).closest("article");
    expect(card).not.toBeNull();
    fireEvent.click(within(card!).getByRole("button", { name: ru.files.retry }));
    await vi.waitFor(() => expect(within(card!).getByRole("status", { name: ru.files.retrying })).toBeInTheDocument());

    resolveRevalidation(Response.json(cachedPage));

    await vi.waitFor(() => {
      expect(within(card!).getByRole("status", { name: ru.files.retrying })).toBeInTheDocument();
      expect(within(card!).queryByText(ru.files.insufficientTokensDescription)).not.toBeInTheDocument();
      expect(observedCache?.getImageFilesFirstPage()?.items).toEqual([activatedQueuedJob]);
    });
  });

  it("keeps a loaded next page when an awaiting-payment image activates in the same render", async () => {
    const awaitingPaymentJob = {
      ...firstSucceededJob,
      id: "f095137a-9e94-4f6d-a4a6-e9820b909e87",
      status: "awaiting_payment" as const,
      prompt: "activate while loading more",
    };
    const activatedQueuedJob = {
      ...awaitingPaymentJob,
      status: "queued" as const,
      updated_at: "2026-08-03T13:00:01Z",
    };
    const nextPageJob = {
      ...secondSucceededJob,
      status: "queued" as const,
      prompt: "newly loaded next page",
    };
    let resolveNextPage: (response: Response) => void = () => {};
    const nextPage = new Promise<Response>((resolve) => {
      resolveNextPage = resolve;
    });
    let resolveActivation: (response: Response) => void = () => {};
    const activation = new Promise<Response>((resolve) => {
      resolveActivation = resolve;
    });
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [awaitingPaymentJob], has_more: true, next_cursor: "next-page" }))
      .mockReturnValueOnce(nextPage);
    vi.mocked(webBrowserMutation).mockReturnValueOnce(activation);

    renderFilesWorkspace();

    await screen.findByText(awaitingPaymentJob.prompt);
    fireEvent.click(screen.getByRole("button", { name: ru.files.loadMore }));
    const card = screen.getByText(awaitingPaymentJob.prompt).closest("article");
    expect(card).not.toBeNull();
    fireEvent.click(within(card!).getByRole("button", { name: ru.files.retry }));
    await vi.waitFor(() => expect(webBrowserMutation).toHaveBeenCalledTimes(1));

    await act(async () => {
      resolveNextPage(Response.json({ items: [nextPageJob], has_more: false, next_cursor: null }));
      resolveActivation(Response.json({ job: activatedQueuedJob }, { status: 200 }));
      await Promise.resolve();
    });

    expect(await screen.findByText(nextPageJob.prompt)).toBeInTheDocument();
    expect(within(card!).getByRole("status", { name: ru.files.retrying })).toBeInTheDocument();
  });

  it("limits simultaneous artifact metadata requests for visible ready cards", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [firstSucceededJob, secondSucceededJob, thirdSucceededJob], has_more: false, next_cursor: null }))
      .mockReturnValue(new Promise<Response>(() => {}));

    renderFilesWorkspace();

    await screen.findByText(thirdSucceededJob.prompt);
    await new Promise<void>((resolve) => window.setTimeout(resolve, 0));

    expect(webBrowserFetch).toHaveBeenCalledTimes(3);
  });

  it("does not start queued preview requests after the files workspace unmounts", async () => {
    let resolveFirstPreview: (response: Response) => void = () => {};
    let resolveSecondPreview: (response: Response) => void = () => {};
    const firstPreview = new Promise<Response>((resolve) => {
      resolveFirstPreview = resolve;
    });
    const secondPreview = new Promise<Response>((resolve) => {
      resolveSecondPreview = resolve;
    });

    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [firstSucceededJob, secondSucceededJob, thirdSucceededJob], has_more: false, next_cursor: null }))
      .mockReturnValueOnce(firstPreview)
      .mockReturnValueOnce(secondPreview);

    const workspace = renderFilesWorkspace();

    await screen.findByText(thirdSucceededJob.prompt);
    await new Promise<void>((resolve) => window.setTimeout(resolve, 0));
    expect(webBrowserFetch).toHaveBeenCalledTimes(3);

    workspace.unmount();
    resolveFirstPreview(Response.json(firstResult));
    resolveSecondPreview(Response.json(secondResult));

    await new Promise<void>((resolve) => window.setTimeout(resolve, 0));
    expect(webBrowserFetch).toHaveBeenCalledTimes(3);
  });

  it("keeps an already loaded preview when local filters rerender its card", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [firstSucceededJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json(firstResult));

    renderFilesWorkspace();

    await screen.findByRole("img", { name: ru.files.generatedImageAlt });
    fireEvent.change(screen.getByRole("searchbox", { name: ru.files.searchLabel }), { target: { value: "night" } });
    fireEvent.change(screen.getByRole("searchbox", { name: ru.files.searchLabel }), { target: { value: "" } });

    expect(webBrowserFetch).toHaveBeenCalledTimes(2);
  });

  it("uses the cursor to append another bounded page", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [firstSucceededJob], has_more: true, next_cursor: "next-page" }))
      .mockResolvedValueOnce(Response.json(firstResult))
      .mockResolvedValueOnce(Response.json({ items: [secondSucceededJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json(secondResult));

    renderFilesWorkspace();

    await screen.findByText(firstSucceededJob.prompt);
    await screen.findByRole("img", { name: ru.files.generatedImageAlt });
    fireEvent.click(screen.getByRole("button", { name: ru.files.loadMore }));

    expect(await screen.findByText(secondSucceededJob.prompt)).toBeInTheDocument();
    expect(webBrowserFetch).toHaveBeenCalledWith("/web/v1/image-jobs?limit=12&cursor=next-page");
  });

  it("does not retry a failed image preview until the user explicitly asks", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [firstSucceededJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json({ error: "Unavailable" }, { status: 503 }));

    renderFilesWorkspace();

    await screen.findByRole("alert");
    expect(webBrowserFetch).toHaveBeenCalledTimes(2);

    fireEvent.click(screen.getByRole("button", { name: ru.files.previewRetry }));
    await vi.waitFor(() => expect(webBrowserFetch).toHaveBeenCalledTimes(3));
  });

  it("renders a cached first file page before delayed revalidation completes", async () => {
    const cachedPage: ImageJobList = {
      items: [{ ...firstSucceededJob, status: "queued", prompt: "cached workspace file" }],
      has_more: false,
      next_cursor: null,
    };
    const refreshedPage: ImageJobList = {
      items: [{ ...firstSucceededJob, status: "queued", prompt: "refreshed workspace file" }],
      has_more: false,
      next_cursor: null,
    };
    let resolveRevalidation: (response: Response) => void = () => {};
    const revalidation = new Promise<Response>((resolve) => {
      resolveRevalidation = resolve;
    });
    vi.mocked(webBrowserFetch).mockReturnValueOnce(revalidation);

    renderFilesWorkspace({ cachePage: cachedPage });

    expect(screen.getByText("cached workspace file")).toBeInTheDocument();
    expect(screen.queryByText(ru.files.loading)).not.toBeInTheDocument();

    resolveRevalidation(Response.json(refreshedPage));

    expect(await screen.findByText("refreshed workspace file")).toBeInTheDocument();
  });

  it("preserves a locally retried child when stale cached-page revalidation completes", async () => {
    const cachedPage: ImageJobList = {
      items: [expiredPreparationJob],
      has_more: false,
      next_cursor: null,
    };
    const serverSiblingJob = {
      ...secondSucceededJob,
      status: "queued" as const,
      prompt: "server sibling from revalidation",
    };
    const staleRevalidationPage: ImageJobList = {
      items: [expiredPreparationJob, serverSiblingJob],
      has_more: false,
      next_cursor: null,
    };
    const retriedSucceededJob = { ...retriedQueuedJob, status: "succeeded" as const };
    let resolveRevalidation: (response: Response) => void = () => {};
    const revalidation = new Promise<Response>((resolve) => {
      resolveRevalidation = resolve;
    });
    vi.mocked(webBrowserFetch)
      .mockReturnValueOnce(revalidation)
      .mockResolvedValueOnce(Response.json({ job: retriedSucceededJob }))
      .mockResolvedValueOnce(Response.json(retriedResult));
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json({ job: retriedQueuedJob }, { status: 200 }));

    renderFilesWorkspace({ cachePage: cachedPage });

    fireEvent.click(screen.getByRole("button", { name: ru.files.retry }));
    await vi.waitFor(() => expect(screen.getByRole("status", { name: ru.files.retrying })).toBeInTheDocument());

    const image = await screen.findByRole("img", { name: ru.files.generatedImageAlt }, { timeout: 3000 });
    expect(image).toHaveAttribute("src", `/web/v1/image-artifacts/${retriedResult.artifacts[0].id}`);

    resolveRevalidation(Response.json(staleRevalidationPage));

    expect(await screen.findByText(serverSiblingJob.prompt)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: ru.files.retry })).not.toBeInTheDocument();
    expect(screen.getByRole("img", { name: ru.files.generatedImageAlt })).toBe(image);
    expect(screen.getByText(serverSiblingJob.prompt)).toBeInTheDocument();
    expect(webBrowserFetch).toHaveBeenCalledTimes(3);
  });

  it("keeps a newer terminal revalidation when an older in-flight retry poll resolves", async () => {
    vi.useFakeTimers();
    try {
      const retryingChild = { ...retriedQueuedJob, prompt: "retry child race result" };
      const cachedPage: ImageJobList = {
        items: [retryingChild],
        has_more: false,
        next_cursor: null,
      };
      const newerSucceededChild = {
        ...retryingChild,
        status: "succeeded" as const,
        updated_at: "2026-08-03T12:02:00Z",
      };
      const olderPollingChild = {
        ...retryingChild,
        status: "provider_processing" as const,
        updated_at: "2026-08-03T12:01:00Z",
      };
      let resolveRevalidation: (response: Response) => void = () => {};
      const revalidation = new Promise<Response>((resolve) => {
        resolveRevalidation = resolve;
      });
      let resolveStalePoll: (response: Response) => void = () => {};
      const stalePoll = new Promise<Response>((resolve) => {
        resolveStalePoll = resolve;
      });
      let retryPollRequests = 0;
      vi.mocked(webBrowserFetch).mockImplementation((path) => {
        if (path === "/web/v1/image-jobs?limit=12") {
          return revalidation;
        }
        if (path === `/web/v1/image-jobs/${retryingChild.id}`) {
          retryPollRequests += 1;
          return retryPollRequests === 1 ? stalePoll : Promise.reject(new Error("Unexpected retry poll."));
        }
        if (path === `/web/v1/image-jobs/${retryingChild.id}/result`) {
          return Promise.resolve(Response.json(retriedResult));
        }
        return Promise.reject(new Error("Unexpected test request."));
      });

      renderFilesWorkspace({
        cachePage: cachedPage,
        retryReplacement: { originalJobID: expiredPreparationJob.id, job: retryingChild },
      });

      act(() => {
        vi.advanceTimersByTime(0);
      });
      expect(screen.getByRole("status", { name: ru.files.retrying })).toBeInTheDocument();
      expect(screen.getByRole("heading", { name: retryingChild.prompt })).toBeInTheDocument();

      act(() => {
        vi.advanceTimersByTime(1501);
      });
      expect(retryPollRequests).toBe(1);

      await act(async () => {
        resolveRevalidation(Response.json({ items: [newerSucceededChild], has_more: false, next_cursor: null }));
        await Promise.resolve();
      });
      expect(screen.getByRole("img", { name: ru.files.generatedImageAlt })).toBeInTheDocument();

      await act(async () => {
        resolveStalePoll(Response.json({ job: olderPollingChild }));
        await Promise.resolve();
      });

      expect(screen.getByRole("img", { name: ru.files.generatedImageAlt })).toBeInTheDocument();
      expect(screen.getByText(ru.files.statusReady)).toBeInTheDocument();
      expect(screen.queryByRole("status", { name: ru.files.retrying })).not.toBeInTheDocument();
      act(() => {
        vi.advanceTimersByTime(3000);
      });
      expect(retryPollRequests).toBe(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("preserves the latest retry child and tracking across a cached workspace remount", async () => {
    const cachedPage: ImageJobList = {
      items: [expiredPreparationJob],
      has_more: false,
      next_cursor: null,
    };
    const staleChild = { ...retriedQueuedJob, updated_at: "2026-08-03T12:01:00Z" };
    const retriedSucceededJob = {
      ...retriedQueuedJob,
      status: "succeeded" as const,
      updated_at: "2026-08-03T12:02:00Z",
    };
    const serverSiblingJob = {
      ...secondSucceededJob,
      status: "queued" as const,
      prompt: "server sibling retained across remount",
    };
    let resolveInitialRevalidation: (response: Response) => void = () => {};
    const initialRevalidation = new Promise<Response>((resolve) => {
      resolveInitialRevalidation = resolve;
    });
    let listRequestCount = 0;
    vi.mocked(webBrowserFetch).mockImplementation((path) => {
      if (path === "/web/v1/image-jobs?limit=12") {
        listRequestCount += 1;
        return listRequestCount === 1 ? initialRevalidation : new Promise<Response>(() => {});
      }
      if (path === `/web/v1/image-jobs/${retriedQueuedJob.id}`) {
        return Promise.resolve(Response.json({ job: retriedSucceededJob }));
      }
      if (path === `/web/v1/image-jobs/${retriedQueuedJob.id}/result`) {
        return Promise.resolve(Response.json(retriedResult));
      }
      return Promise.reject(new Error("Unexpected test request."));
    });
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json({ job: retriedQueuedJob }, { status: 200 }));
    const workspace = render(
      <WorkspaceDataCacheProvider>
        <WorkspaceDataCacheSeed page={cachedPage} />
        <FilesWorkspace />
      </WorkspaceDataCacheProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: ru.files.retry }));
    await vi.waitFor(() => {
      expect(screen.getByRole("status", { name: ru.files.retrying })).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: ru.files.retry })).not.toBeInTheDocument();
    });

    resolveInitialRevalidation(Response.json({
      items: [expiredPreparationJob, staleChild, serverSiblingJob],
      has_more: false,
      next_cursor: null,
    }));
    expect(await screen.findByText(serverSiblingJob.prompt)).toBeInTheDocument();

    workspace.rerender(<WorkspaceDataCacheProvider>{null}</WorkspaceDataCacheProvider>);
    workspace.rerender(
      <WorkspaceDataCacheProvider>
        <FilesWorkspace />
      </WorkspaceDataCacheProvider>,
    );

    expect(screen.queryByRole("button", { name: ru.files.retry })).not.toBeInTheDocument();
    expect(screen.getAllByRole("heading", { name: retriedQueuedJob.prompt })).toHaveLength(1);
    expect(await screen.findByRole("img", { name: ru.files.generatedImageAlt }, { timeout: 3000 })).toBeInTheDocument();
    expect(webBrowserFetch).toHaveBeenCalledWith(`/web/v1/image-jobs/${retriedQueuedJob.id}`, expect.anything());
  });

  it("deduplicates an existing retry child when idempotent retry replaces the original card", async () => {
    const existingChild = {
      ...retriedQueuedJob,
      status: "provider_processing" as const,
      updated_at: "2026-08-03T12:05:00Z",
    };
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({
        items: [expiredPreparationJob, existingChild],
        has_more: false,
        next_cursor: null,
      }))
      .mockReturnValue(new Promise<Response>(() => {}));
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json({ job: retriedQueuedJob }, { status: 200 }));

    renderFilesWorkspace();

    fireEvent.click(await screen.findByRole("button", { name: ru.files.retry }));

    await vi.waitFor(() => expect(screen.queryByRole("button", { name: ru.files.retry })).not.toBeInTheDocument());
    expect(screen.getAllByRole("heading", { name: retriedQueuedJob.prompt })).toHaveLength(1);
    expect(screen.getByRole("status", { name: ru.files.retrying })).toBeInTheDocument();
  });

  it("retains a tracked retry child when a full stale first page contains neither retry ID", async () => {
    const unrelatedJobs = Array.from({ length: 12 }, (_, index) => ({
      ...firstSucceededJob,
      id: `00000000-0000-4000-8000-${String(index + 1).padStart(12, "0")}`,
      status: "queued" as const,
      prompt: `unrelated stale job ${index + 1}`,
    }));
    let resolveRevalidation: (response: Response) => void = () => {};
    const revalidation = new Promise<Response>((resolve) => {
      resolveRevalidation = resolve;
    });
    const retriedSucceededJob = {
      ...retriedQueuedJob,
      status: "succeeded" as const,
      updated_at: "2026-08-03T12:10:00Z",
    };
    vi.mocked(webBrowserFetch)
      .mockReturnValueOnce(revalidation)
      .mockResolvedValueOnce(Response.json({ job: retriedSucceededJob }))
      .mockResolvedValueOnce(Response.json(retriedResult));
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json({ job: retriedQueuedJob }, { status: 200 }));

    renderFilesWorkspace({ cachePage: {
      items: [expiredPreparationJob],
      has_more: false,
      next_cursor: null,
    } });

    fireEvent.click(screen.getByRole("button", { name: ru.files.retry }));
    await vi.waitFor(() => expect(screen.queryByRole("button", { name: ru.files.retry })).not.toBeInTheDocument());

    resolveRevalidation(Response.json({ items: unrelatedJobs, has_more: false, next_cursor: null }));

    expect(await screen.findByText(unrelatedJobs[0].prompt)).toBeInTheDocument();
    expect(screen.getAllByRole("heading", { name: retriedQueuedJob.prompt })).toHaveLength(1);
    expect(screen.getAllByRole("heading", { level: 2 })).toHaveLength(12);
    expect(screen.getByRole("status", { name: ru.files.retrying })).toBeInTheDocument();
    expect(await screen.findByRole("img", { name: ru.files.generatedImageAlt }, { timeout: 3000 })).toBeInTheDocument();
    expect(screen.getAllByRole("heading", { name: retriedQueuedJob.prompt })).toHaveLength(1);
  });

  it("resumes polling only the returned retry job when the files tab becomes visible", async () => {
    let visibilityState: DocumentVisibilityState = "hidden";
    vi.spyOn(document, "visibilityState", "get").mockImplementation(() => visibilityState);
    const retriedSucceededJob = { ...retriedQueuedJob, status: "succeeded" as const };
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [expiredPreparationJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json({ job: retriedSucceededJob }))
      .mockResolvedValueOnce(Response.json(retriedResult));
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json({ job: retriedQueuedJob }, { status: 200 }));

    renderFilesWorkspace();

    fireEvent.click(await screen.findByRole("button", { name: ru.files.retry }));
    await vi.waitFor(() => {
      expect(screen.getByRole("status", { name: ru.files.retrying })).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: ru.files.retry })).not.toBeInTheDocument();
    });
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);

    visibilityState = "visible";
    fireEvent(document, new Event("visibilitychange"));

    expect(await screen.findByRole("img", { name: ru.files.generatedImageAlt })).toBeInTheDocument();
    expect(webBrowserFetch).toHaveBeenNthCalledWith(2, `/web/v1/image-jobs/${retriedQueuedJob.id}`, expect.anything());
    expect(webBrowserFetch).toHaveBeenCalledTimes(3);
  });

  it("retries only the clicked expired card in place and marks it busy while the mutation is unresolved", async () => {
    const anotherExpiredJob = {
      ...expiredPreparationJob,
      id: "cd343230-4037-4cf7-99f3-af572b9eac96",
      prompt: "another expired prompt",
    };
    let settleRetry: (response: Response) => void = () => {};
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json({
      items: [expiredPreparationJob, anotherExpiredJob],
      has_more: false,
      next_cursor: null,
    }));
    vi.mocked(webBrowserMutation).mockReturnValueOnce(new Promise<Response>((resolve) => {
      settleRetry = resolve;
    }));
    window.history.replaceState(null, "", "/app/files");

    renderFilesWorkspace();

    const firstCard = (await screen.findByText(expiredPreparationJob.prompt)).closest("article");
    const secondCard = screen.getByText(anotherExpiredJob.prompt).closest("article");
    expect(firstCard).not.toBeNull();
    expect(secondCard).not.toBeNull();
    const firstRetry = within(firstCard!).getByRole("button", { name: ru.files.retry });
    fireEvent.click(firstRetry);

    expect(window.location.pathname).toBe("/app/files");
    expect(screen.queryByRole("link", { name: ru.files.retry })).not.toBeInTheDocument();
    expect(webBrowserMutation).toHaveBeenCalledWith(`/web/v1/image-jobs/${expiredPreparationJob.id}/retry`, {
      method: "POST",
    });
    expect(firstCard).toHaveAttribute("aria-busy", "true");
    expect(firstRetry).toBeDisabled();
    expect(within(firstCard!).getByRole("status", { name: ru.files.retrying })).toBeInTheDocument();
    expect(secondCard).not.toHaveAttribute("aria-busy", "true");
    expect(within(secondCard!).getByRole("button", { name: ru.files.retry })).toBeEnabled();

    settleRetry(Response.json({ job: retriedQueuedJob }, { status: 200 }));
  });

  it("replaces the expired card, polls only the returned retry job, and renders its preview without refetching the list", async () => {
    const retriedSucceededJob = { ...retriedQueuedJob, status: "succeeded" as const };
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [expiredPreparationJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json({ job: retriedSucceededJob }))
      .mockResolvedValueOnce(Response.json(retriedResult));
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json({ job: retriedQueuedJob }, { status: 200 }));

    renderFilesWorkspace();

    fireEvent.click(await screen.findByRole("button", { name: ru.files.retry }));
    const busyCard = (await screen.findByText(retriedQueuedJob.prompt)).closest("article");
    expect(busyCard).toHaveAttribute("aria-busy", "true");

    const image = await screen.findByRole("img", { name: ru.files.generatedImageAlt }, { timeout: 3000 });
    expect(webBrowserFetch).toHaveBeenNthCalledWith(1, "/web/v1/image-jobs?limit=12");
    expect(webBrowserFetch).toHaveBeenNthCalledWith(2, `/web/v1/image-jobs/${retriedQueuedJob.id}`, expect.anything());
    expect(webBrowserFetch).toHaveBeenNthCalledWith(3, `/web/v1/image-jobs/${retriedQueuedJob.id}/result`);
    expect(webBrowserFetch).toHaveBeenCalledTimes(3);
    expect(webBrowserFetch).not.toHaveBeenCalledWith(`/web/v1/image-jobs/${expiredPreparationJob.id}`, expect.anything());
    expect(image).toHaveAttribute("src", `/web/v1/image-artifacts/${retriedResult.artifacts[0].id}`);
    expect(image.closest("article")).not.toHaveAttribute("aria-busy", "true");
  });

  it("replaces the expired card with the returned awaiting-payment retry without redirecting", async () => {
    const awaitingPaymentRetry = { ...retriedQueuedJob, status: "awaiting_payment" as const };
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json({
      items: [expiredPreparationJob],
      has_more: false,
      next_cursor: null,
    }));
    vi.mocked(webBrowserMutation).mockResolvedValueOnce(Response.json({ job: awaitingPaymentRetry }, { status: 402 }));
    window.history.replaceState(null, "", "/app/files");

    renderFilesWorkspace();

    fireEvent.click(await screen.findByRole("button", { name: ru.files.retry }));

    expect(await screen.findByText(ru.files.statusInsufficientTokens)).toBeInTheDocument();
    expect(screen.getByText(ru.files.insufficientTokensDescription)).toBeInTheDocument();
    expect(window.location.pathname).toBe("/app/files");
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);
  });

  it("restores the retry button when the inline retry mutation fails", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(Response.json({
      items: [expiredPreparationJob],
      has_more: false,
      next_cursor: null,
    }));
    vi.mocked(webBrowserMutation).mockRejectedValueOnce(new Error("raw backend detail"));

    renderFilesWorkspace();

    fireEvent.click(await screen.findByRole("button", { name: ru.files.retry }));

    expect(await screen.findByRole("button", { name: ru.files.retry })).toBeEnabled();
    expect(screen.queryByText("raw backend detail")).not.toBeInTheDocument();
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);
  });
});
