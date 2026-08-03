import { StrictMode } from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserFetch: vi.fn(),
}));

import { ru } from "@/i18n/ru";
import { webBrowserFetch } from "@/lib/web-api/browser";
import type { ImageJobList } from "@/lib/web-api/contracts";
import {
  useWorkspaceDataCache,
  WorkspaceDataCacheProvider,
} from "@/features/workspace/WorkspaceDataCache/WorkspaceDataCache";

import { FilesWorkspace } from "./FilesWorkspace";

function WorkspaceDataCacheSeed({ page }: { page?: ImageJobList }) {
  const cache = useWorkspaceDataCache();

  if (page !== undefined) {
    cache.setImageFilesFirstPage(page);
  }

  return null;
}

function renderFilesWorkspace({ cachePage, strictMode = false }: { cachePage?: ImageJobList; strictMode?: boolean } = {}) {
  const workspace = <FilesWorkspace />;

  return render(
    <WorkspaceDataCacheProvider>
      <WorkspaceDataCacheSeed page={cachePage} />
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

describe("FilesWorkspace", () => {
  afterEach(() => {
    cleanup();
    vi.resetAllMocks();
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
    expect(screen.getByRole("link", { name: "Повторить" })).toHaveAttribute(
      "href",
      "/app/image?model=gpt-image-2&quality=1K&prompt=night+city+after+rain",
    );
    expect(screen.queryAllByText(ru.files.noReadyArtifact)).toHaveLength(0);
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
});
