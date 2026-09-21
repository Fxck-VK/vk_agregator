import { StrictMode } from "react";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserFetch: vi.fn(),
  webBrowserMutation: vi.fn(),
}));

import { ru } from "@/i18n/ru";
import { webBrowserFetch } from "@/lib/web-api/browser";
import type { ImageJobList } from "@/lib/web-api/contracts";
import {
  useWorkspaceDataCache,
  WorkspaceDataCacheProvider,
} from "@/features/workspace/WorkspaceDataCache/WorkspaceDataCache";

import { FilesWorkspace } from "./FilesWorkspace";

function WorkspaceDataCacheSeed({
  page,
}: {
  page?: ImageJobList;
}) {
  const cache = useWorkspaceDataCache();

  if (page !== undefined) {
    cache.setImageFilesFirstPage(page);
  }

  return null;
}

function renderFilesWorkspace({
  cachePage,
  initialCategory,
  strictMode = false,
}: {
  cachePage?: ImageJobList;
  initialCategory?: "all" | "images" | "reports" | "presentations" | "video" | "uploads";
  strictMode?: boolean;
} = {}) {
  const workspace = <FilesWorkspace initialCategory={initialCategory} />;

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
    Reflect.deleteProperty(navigator, "clipboard");
    Reflect.deleteProperty(navigator, "share");
    vi.unstubAllGlobals();
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
    expect(webBrowserFetch).toHaveBeenNthCalledWith(2, `/web/v1/image-jobs/${firstSucceededJob.id}/result`, expect.objectContaining({ signal: expect.any(AbortSignal) }));
    expect(image).toHaveAttribute("src", "/web/v1/image-artifacts/6ca96a58-a902-4f23-a92a-6726e1a0cd20?preview=1");
    expect(screen.queryByText("https://objects.example.test/private-key")).not.toBeInTheDocument();
  });

  it("opens a generated file in the shared modal backdrop and closes it from the dialog", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [firstSucceededJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json(firstResult));

    renderFilesWorkspace();

    const previewTrigger = await screen.findByRole("button", {
      name: `Открыть файл: ${firstSucceededJob.prompt}`,
    });
    previewTrigger.focus();
    fireEvent.click(previewTrigger);

    const dialog = screen.getByRole("dialog", {
      name: `Просмотр файла: ${firstSucceededJob.prompt}`,
    });
    expect(within(dialog).getByRole("img", { name: firstSucceededJob.prompt })).toHaveAttribute(
      "src",
      "/web/v1/image-artifacts/6ca96a58-a902-4f23-a92a-6726e1a0cd20",
    );
    expect(within(dialog).getByText(firstSucceededJob.model_name)).toBeInTheDocument();
    expect(within(dialog).getByText(firstSucceededJob.prompt)).toBeInTheDocument();
    expect(within(dialog).getByText(firstSucceededJob.image_quality)).toBeInTheDocument();
    expect(within(dialog).getByRole("link", { name: "Скачать файл" })).toHaveAttribute("download");
    expect(within(dialog).getByRole("button", { name: "Поделиться файлом" })).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Оживить" }))
      .toHaveAttribute("aria-pressed", "false");
    expect(within(dialog).getByRole("button", { name: "Оживить" }))
      .not.toHaveAttribute("title");

    fireEvent.click(within(dialog).getByRole("button", { name: "Закрыть предпросмотр" }));
    const backdrop = screen.getByTestId("file-preview-backdrop");
    expect(backdrop).toHaveAttribute("data-state", "closing");

    fireEvent.animationEnd(backdrop, { animationName: "modalBackdropOut" });
    expect(screen.queryByRole("dialog", {
      name: `Просмотр файла: ${firstSucceededJob.prompt}`,
    })).toBeNull();
    expect(previewTrigger).toHaveFocus();
  });

  it("opens the clicked file inside an ordered thumbnail gallery", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({
        items: [firstSucceededJob, secondSucceededJob],
        has_more: false,
        next_cursor: null,
      }))
      .mockResolvedValueOnce(Response.json(firstResult))
      .mockResolvedValueOnce(Response.json(secondResult));

    renderFilesWorkspace();

    const openButtons = await screen.findAllByRole("button", { name: /^Открыть файл:/ });
    fireEvent.click(openButtons[1]!);

    const dialog = screen.getByRole("dialog", {
      name: `Просмотр файла: ${secondSucceededJob.prompt}`,
    });
    expect(within(dialog).getAllByRole("button", { name: /^Выбрать файл:/ })).toHaveLength(2);
    expect(within(dialog).getByRole("img", { name: secondSucceededJob.prompt })).toHaveAttribute(
      "src",
      `/web/v1/image-artifacts/${secondResult.artifacts[0]!.id}`,
    );

    fireEvent.click(within(dialog).getByRole("button", {
      name: `Выбрать файл: ${firstSucceededJob.prompt}`,
    }));

    expect(screen.getByRole("dialog", {
      name: `Просмотр файла: ${firstSucceededJob.prompt}`,
    })).toBeInTheDocument();
    expect(screen.getByRole("img", { name: firstSucceededJob.prompt })).toHaveAttribute(
      "src",
      `/web/v1/image-artifacts/${firstResult.artifacts[0]!.id}`,
    );
  });

  it("loads off-screen successful files when the gallery opens", async () => {
    const intersectionCallbacks: IntersectionObserverCallback[] = [];
    class TestIntersectionObserver {
      constructor(callback: IntersectionObserverCallback) {
        intersectionCallbacks.push(callback);
      }

      disconnect() {}
      observe() {}
      takeRecords() { return []; }
      unobserve() {}
    }
    vi.stubGlobal("IntersectionObserver", TestIntersectionObserver);
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({
        items: [firstSucceededJob, secondSucceededJob],
        has_more: false,
        next_cursor: null,
      }))
      .mockResolvedValueOnce(Response.json(firstResult))
      .mockReturnValueOnce(new Promise<Response>(() => {}));

    renderFilesWorkspace();

    await waitFor(() => expect(intersectionCallbacks).toHaveLength(2));
    intersectionCallbacks[0]!([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver);
    fireEvent.click(await screen.findByRole("button", {
      name: `Открыть файл: ${firstSucceededJob.prompt}`,
    }));

    await waitFor(() => {
      expect(webBrowserFetch).toHaveBeenCalledWith(`/web/v1/image-jobs/${secondSucceededJob.id}/result`, expect.objectContaining({ signal: expect.any(AbortSignal) }));
    });
    expect(screen.getByRole("button", {
      name: `Загружаем файл: ${secondSucceededJob.prompt}`,
    })).toBeDisabled();
  });

  it("copies the selected artifact URL when the platform share sheet is unavailable", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    Object.defineProperty(navigator, "share", {
      configurable: true,
      value: undefined,
    });
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [firstSucceededJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json(firstResult));

    renderFilesWorkspace();

    fireEvent.click(await screen.findByRole("button", {
      name: `Открыть файл: ${firstSucceededJob.prompt}`,
    }));
    fireEvent.click(screen.getByRole("button", { name: "Поделиться файлом" }));

    const artifactPath = "/web/v1/image-artifacts/6ca96a58-a902-4f23-a92a-6726e1a0cd20";
    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith(new URL(artifactPath, window.location.origin).toString());
    });
    expect(screen.getByRole("status")).toHaveTextContent("Ссылка скопирована");
  });

  it("keeps image previews available in React Strict Mode", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [firstSucceededJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json(firstResult));

    renderFilesWorkspace({ strictMode: true });

    expect(await screen.findByRole("img", { name: ru.files.generatedImageAlt })).toHaveAttribute(
      "src",
      "/web/v1/image-artifacts/6ca96a58-a902-4f23-a92a-6726e1a0cd20?preview=1",
    );
  });

  it("shows only successfully generated files without search or status controls", async () => {
    const pendingJob = { ...secondSucceededJob, status: "queued" as const, prompt: "queued forest image" };
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [firstSucceededJob, pendingJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json(firstResult));

    renderFilesWorkspace();

    await screen.findByText(firstSucceededJob.prompt);
    expect(screen.queryByText(pendingJob.prompt)).not.toBeInTheDocument();
    expect(screen.getByText(firstSucceededJob.prompt)).toBeInTheDocument();
    expect(screen.queryByRole("searchbox", { name: ru.files.searchLabel })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: ru.files.statusFilterLabel })).not.toBeInTheDocument();
    expect(screen.queryByText(ru.files.loadedScopeNotice)).not.toBeInTheDocument();
  });

  it("shows the library empty state when every loaded job is unfinished", async () => {
    const pendingJob = { ...firstSucceededJob, status: "queued" as const, prompt: "queued forest image" };
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(
      Response.json({ items: [pendingJob], has_more: false, next_cursor: null }),
    );

    renderFilesWorkspace();

    expect(await screen.findByRole("heading", { name: ru.files.emptyLibraryTitle })).toBeInTheDocument();
    expect(screen.queryByText(pendingJob.prompt)).not.toBeInTheDocument();
  });

  it("keeps image files available while future categories show their own empty state without another request", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [firstSucceededJob], has_more: false, next_cursor: null }))
      .mockResolvedValueOnce(Response.json(firstResult));

    renderFilesWorkspace();

    await screen.findByRole("img", { name: ru.files.generatedImageAlt });
    expect(screen.getByText(firstSucceededJob.prompt)).toBeInTheDocument();
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

    await vi.waitFor(() => {
      expect(screen.getAllByRole("img", { name: ru.files.generatedImageAlt })).toHaveLength(2);
    });
    expect(screen.getByText(secondSucceededJob.prompt)).toBeInTheDocument();
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
      items: [{ ...firstSucceededJob, prompt: "cached workspace file" }],
      has_more: false,
      next_cursor: null,
    };
    const refreshedPage: ImageJobList = {
      items: [{ ...firstSucceededJob, prompt: "refreshed workspace file" }],
      has_more: false,
      next_cursor: null,
    };
    let resolveRevalidation: (response: Response) => void = () => {};
    const revalidation = new Promise<Response>((resolve) => {
      resolveRevalidation = resolve;
    });
    vi.mocked(webBrowserFetch).mockImplementation((path) => {
      if (path === "/web/v1/image-jobs?limit=12") {
        return revalidation;
      }
      if (path === `/web/v1/image-jobs/${firstSucceededJob.id}/result`) {
        return Promise.resolve(Response.json(firstResult));
      }
      return Promise.reject(new Error("Unexpected test request."));
    });

    renderFilesWorkspace({ cachePage: cachedPage });

    expect(screen.getByText("cached workspace file")).toBeInTheDocument();
    expect(screen.queryByText(ru.files.loading)).not.toBeInTheDocument();

    resolveRevalidation(Response.json(refreshedPage));

    expect(await screen.findByText("refreshed workspace file")).toBeInTheDocument();
  });

});
