import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { StrictMode } from "react";
import { describe, expect, it, vi } from "vitest";

import { LocaleProvider } from "@/i18n/LocaleProvider";
import { WorkspaceDataCacheProvider, useWorkspaceDataCache, type WorkspaceDataCache } from "@/features/workspace/WorkspaceDataCache/WorkspaceDataCache";

import { MusicApiError, type MusicJob, type MusicJobList, type MusicJobResult } from "../music-api";
import { MusicWorkspaceController, type MusicWorkspaceAPI } from "./MusicWorkspaceController";
import type { MusicModelCatalog } from "../music-model-catalog";

function apiStub(overrides: Partial<MusicWorkspaceAPI> = {}): MusicWorkspaceAPI {
  return {
    activateMusicJob: vi.fn(),
    loadMusicJob: vi.fn(),
    loadMusicJobResult: vi.fn(),
    loadMusicJobs: vi.fn().mockResolvedValue({ has_more: false, items: [], next_cursor: null }),
    prepareMusicJob: vi.fn(),
    uploadMusicInput: vi.fn(),
    ...overrides,
  };
}

const enabledCatalog: MusicModelCatalog = {
  defaultModelId: "suno_v6",
  models: [
    {
      availability: "available",
      description: "Verified music model",
      enabled: true,
      id: "suno_v6",
      name: "Suno V6",
      operations: [
        {
          enabled: true,
          estimateCredits: 5,
          id: "generate",
          maxEstimateCredits: 99,
          supportsCustomModel: true,
          supportsPersona: true,
          supportsAudioFormat: true,
          supportsMaxMode: true,
        },
      ],
    },
  ],
};

const pendingCatalog: MusicModelCatalog = {
  defaultModelId: "suno_v6",
  models: [
    {
      availability: "unverified",
      description: "Pending music model",
      enabled: false,
      id: "suno_v6",
      name: "Suno V6",
      operations: [
        {
          enabled: false,
          estimateCredits: 5,
          id: "generate",
          statusReason: "pending verification",
        },
      ],
      statusReason: "Модель ждёт отдельной проверки",
    },
  ],
};

const oldMusicJob: MusicJob = {
  action: "generate", cost_estimate: 37, created_at: "2026-09-16T10:00:00Z",
  id: "33333333-3333-4333-8333-333333333333", model_id: "suno_v6", status: "succeeded",
};
const newMusicJob: MusicJob = { ...oldMusicJob, id: "44444444-4444-4444-8444-444444444444", created_at: "2026-09-16T11:00:00Z" };
function audioResult(job: MusicJob, title: string): MusicJobResult {
  return {
    artifacts: [], job_id: job.id,
    tracks: [{ audio_index: 1, job_id: job.id, title, audio_url: `/web/v1/music-artifacts/${job.id}`, duration_sec: 120 }],
  };
}
const loadedCatalog = () => Promise.resolve(enabledCatalog);

describe("MusicWorkspaceController", () => {
  it("opens an owned track from My Files and only prepares continuation after a user action", async () => {
    const catalog = { ...enabledCatalog, models: enabledCatalog.models.map(model => ({ ...model, operations: [...model.operations,
      { enabled: true, id: "extend" as const, estimateCredits: 30, requirement: { minTracks: 1, maxTracks: 1 } },
    ] })) };
    const api = apiStub({
      loadMusicJob: vi.fn().mockResolvedValue(oldMusicJob),
      loadMusicJobResult: vi.fn().mockResolvedValue(audioResult(oldMusicJob, "Source song")),
      prepareMusicJob: vi.fn().mockRejectedValue(new MusicApiError("synthetic", 400)),
    });
    render(<MusicWorkspaceController api={api} catalogLoader={() => Promise.resolve(catalog)}
      requestedContinuation={{ modelId: "suno_v6", jobId: oldMusicJob.id, audioIndex: 1, continueAtSec: 45 }} />);
    await screen.findByRole("heading", { name: "Продлить" });
    expect(api.prepareMusicJob).not.toHaveBeenCalled();
    expect(api.activateMusicJob).not.toHaveBeenCalled();
    fireEvent.change(screen.getByRole("textbox", { name: "Как продолжить трек" }), { target: { value: "Continue with soft piano" } });
    fireEvent.click(screen.getByRole("button", { name: "Продолжить трек" }));
    await waitFor(() => expect(api.prepareMusicJob).toHaveBeenCalledOnce());
    expect(api.prepareMusicJob).toHaveBeenCalledWith(expect.objectContaining({
      model_id: "suno_v6", sources: [{ job_id: oldMusicJob.id, audio_index: 1 }],
      music: expect.objectContaining({ action: "extend", custom: false, continue_at_sec: 45, gpt_description: "Continue with soft piano" }),
    }), expect.any(String));
    expect(api.activateMusicJob).not.toHaveBeenCalled();
  });

  it("unlocks continuation after a price timeout and manually retries the same quote key", async () => {
    const catalog = { ...enabledCatalog, models: enabledCatalog.models.map(model => ({ ...model, operations: [...model.operations,
      { enabled: true, id: "extend" as const, estimateCredits: 30, requirement: { minTracks: 1, maxTracks: 1 } },
    ] })) };
    const api = apiStub({
      loadMusicJobs: vi.fn().mockResolvedValue({ has_more: false, items: [oldMusicJob], next_cursor: null }),
      loadMusicJobResult: vi.fn().mockResolvedValue(audioResult(oldMusicJob, "Source song")),
      prepareMusicJob: vi.fn().mockRejectedValueOnce(new MusicApiError("timeout", 408)).mockResolvedValueOnce({
        balance: 100, can_afford: true, job: { ...newMusicJob, action: "extend", status: "prepared", cost_estimate: 30 },
      }),
    });
    render(<MusicWorkspaceController api={api} catalogLoader={() => Promise.resolve(catalog)} />);
    await screen.findByRole("heading", { name: "Source song" });
    fireEvent.change(screen.getByRole("slider", { name: "Перемотка: Source song" }), { target: { value: "45" } });
    fireEvent.click(screen.getByRole("button", { name: "Действия: Source song" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Продолжить трек" }));
    const input = screen.getByRole("textbox", { name: "Как продолжить трек" });
    fireEvent.change(input, { target: { value: "synthetic continuation" } });
    fireEvent.click(screen.getByRole("button", { name: "Продолжить трек" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Сервер долго не отвечает. Повторите попытку; списания не было.");
    expect(input).toHaveValue("synthetic continuation");
    expect(screen.getByRole("button", { name: "Продолжить трек" })).toBeEnabled();
    expect(api.prepareMusicJob).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: "Продолжить трек" }));
    await screen.findByRole("heading", { name: "Подтверждение запуска" });
    const calls = vi.mocked(api.prepareMusicJob).mock.calls;
    expect(calls).toHaveLength(2);
    expect(calls[1][1]).toBe(calls[0][1]);
    expect(calls[1][0]).toEqual(calls[0][0]);
    expect(calls[1][0]).toMatchObject({ sources: [{ job_id: oldMusicJob.id, audio_index: 1 }], music: { continue_at_sec: 45 } });
    expect(api.activateMusicJob).not.toHaveBeenCalled();
  });

  it("rejects a missing or foreign continuation source without preparing or activating", async () => {
    const catalog = { ...enabledCatalog, models: enabledCatalog.models.map(model => ({ ...model, operations: [...model.operations,
      { enabled: true, id: "extend" as const, requirement: { minTracks: 1, maxTracks: 1 } },
    ] })) };
    const api = apiStub({ loadMusicJob: vi.fn().mockRejectedValue(new MusicApiError("not found", 404)) });
    render(<MusicWorkspaceController api={api} catalogLoader={() => Promise.resolve(catalog)}
      requestedContinuation={{ modelId: "suno_v6", jobId: oldMusicJob.id, audioIndex: 1 }} />);
    await screen.findByText("Не удалось открыть исходный трек для продолжения.");
    expect(api.prepareMusicJob).not.toHaveBeenCalled();
    expect(api.activateMusicJob).not.toHaveBeenCalled();
  });
  it("keeps old tracks and polling state after disconnect and resumes only reads on reconnect", async () => {
    vi.useFakeTimers();
    let failRead!: (reason: Error) => void;
    const pendingJob: MusicJob = { ...newMusicJob, status: "queued" };
    const api = apiStub({
      loadMusicJobs: vi.fn().mockResolvedValue({ items: [oldMusicJob, pendingJob], has_more: false, next_cursor: null }),
      loadMusicJob: vi.fn().mockImplementationOnce(() => new Promise((_resolve, reject) => { failRead = reject; }))
        .mockResolvedValue(newMusicJob),
      loadMusicJobResult: vi.fn(async id => audioResult(id === oldMusicJob.id ? oldMusicJob : newMusicJob, id === oldMusicJob.id ? "Старый трек" : "Новый трек")),
    });
    const { unmount } = render(<MusicWorkspaceController api={api} catalogLoader={loadedCatalog} />);
    try {
      await act(async () => { await vi.advanceTimersByTimeAsync(0); });
      await act(async () => { await vi.advanceTimersByTimeAsync(0); });
      expect(screen.getByText("Старый трек")).toBeVisible();
      expect(api.loadMusicJob).toHaveBeenCalledTimes(1);
      await act(async () => {
        Object.defineProperty(navigator, "onLine", { configurable: true, value: false });
        window.dispatchEvent(new Event("offline"));
        failRead(new TypeError("network disconnected"));
        await vi.advanceTimersByTimeAsync(60000);
      });
      expect(screen.getByText("Старый трек")).toBeVisible();
      expect(api.loadMusicJob).toHaveBeenCalledTimes(1);
      expect(screen.queryByRole("alert")).toBeNull();
      await act(async () => {
        Object.defineProperty(navigator, "onLine", { configurable: true, value: true });
        window.dispatchEvent(new Event("online"));
        window.dispatchEvent(new Event("online"));
      });
      expect(api.loadMusicJob).toHaveBeenCalledTimes(2);
      expect(screen.getByRole("heading", { name: "Новый трек", level: 3 })).toBeVisible();
      expect(screen.getByText("Старый трек")).toBeVisible();
      expect(api.activateMusicJob).not.toHaveBeenCalled();
      expect(api.prepareMusicJob).not.toHaveBeenCalled();
    } finally {
      unmount();
      Object.defineProperty(navigator, "onLine", { configurable: true, value: true });
      vi.useRealTimers();
    }
  });
  it("keeps cached tracks and lyrics visible when history revalidation fails", async () => {
    const api = apiStub({
      loadMusicJobs: vi.fn()
        .mockResolvedValueOnce({ items: [oldMusicJob], has_more: false, next_cursor: null })
        .mockRejectedValueOnce(new Error("offline")),
      loadMusicJobResult: vi.fn().mockResolvedValue({
        ...audioResult(oldMusicJob, "Cached track"),
        lyrics: [{ text: "Cached lyrics", title: "Cached song" }],
      }),
    });
    const workspace = (open: boolean) => (
      <WorkspaceDataCacheProvider>{open ? <MusicWorkspaceController api={api} catalogLoader={loadedCatalog} /> : null}</WorkspaceDataCacheProvider>
    );
    const view = render(workspace(true));
    await screen.findByRole("heading", { name: "Cached track" });
    fireEvent.click(screen.getByRole("button", { name: "Действия: Cached track" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Текст песни" }));
    await screen.findByText("Cached lyrics");
    view.rerender(workspace(false));
    view.rerender(workspace(true));
    expect(screen.getByRole("heading", { name: "Cached track" })).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Действия: Cached track" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Текст песни" }));
    expect(screen.getByText("Cached lyrics")).toBeVisible();
    await waitFor(() => expect(api.loadMusicJobs).toHaveBeenCalledTimes(2));
    expect(screen.getByRole("heading", { name: "Cached track" })).toBeVisible();
    expect(api.loadMusicJobResult).toHaveBeenCalledTimes(1);
  });

  it("aborts reads on account change and does not cache or show a late old-account result", async () => {
    let finishOldResult!: (result: MusicJobResult) => void;
    let oldCache!: WorkspaceDataCache;
    function CaptureCache() {
      oldCache = useWorkspaceDataCache();
      return null;
    }
    const oldApi = apiStub({
      loadMusicJobs: vi.fn().mockResolvedValue({ items: [oldMusicJob], has_more: false, next_cursor: null }),
      loadMusicJobResult: vi.fn(() => new Promise<MusicJobResult>(resolve => { finishOldResult = resolve; })),
    });
    const newApi = apiStub();
    const workspace = (owner: "old" | "new") => (
      <WorkspaceDataCacheProvider key={owner}>
        {owner === "old" ? <CaptureCache /> : null}
        <MusicWorkspaceController api={owner === "old" ? oldApi : newApi} catalogLoader={loadedCatalog} />
      </WorkspaceDataCacheProvider>
    );
    const view = render(workspace("old"));
    await waitFor(() => expect(oldApi.loadMusicJobResult).toHaveBeenCalledTimes(1));
    const oldSignal = vi.mocked(oldApi.loadMusicJobResult).mock.calls[0][1]!;
    view.rerender(workspace("new"));
    expect(oldSignal.aborted).toBe(true);
    await act(async () => { finishOldResult(audioResult(oldMusicJob, "Private old track")); });
    expect(screen.queryByRole("heading", { name: "Private old track" })).not.toBeInTheDocument();
    expect(oldCache.musicResults.getAll()).toEqual([]);
    expect(newApi.loadMusicJobResult).not.toHaveBeenCalled();
  });

  it("loads completed history after StrictMode effect replay without duplicate result reads", async () => {
    const api = apiStub({
      loadMusicJobs: vi.fn().mockResolvedValue({ items: [oldMusicJob], has_more: false, next_cursor: null }),
      loadMusicJobResult: vi.fn().mockResolvedValue(audioResult(oldMusicJob, "Strict track")),
    });
    render(<StrictMode><WorkspaceDataCacheProvider><MusicWorkspaceController api={api} catalogLoader={loadedCatalog} /></WorkspaceDataCacheProvider></StrictMode>);
    await screen.findByRole("heading", { name: "Strict track" });
    expect(api.loadMusicJobResult).toHaveBeenCalledTimes(1);
    expect(vi.mocked(api.loadMusicJobResult).mock.calls[0][1]?.aborted).toBe(false);
  });

  it("restores completed tracks immediately on return and fetches only newly completed results", async () => {
    let finishHistory!: (page: MusicJobList) => void;
    const api = apiStub({
      loadMusicJobs: vi.fn()
        .mockResolvedValueOnce({ items: [oldMusicJob], has_more: false, next_cursor: null })
        .mockImplementationOnce(() => new Promise(resolve => { finishHistory = resolve; })),
      loadMusicJobResult: vi.fn(async id => audioResult(id === oldMusicJob.id ? oldMusicJob : newMusicJob, id === oldMusicJob.id ? "Old track" : "New track")),
    });
    const workspace = (open: boolean) => (
      <WorkspaceDataCacheProvider>{open ? <MusicWorkspaceController api={api} catalogLoader={loadedCatalog} /> : null}</WorkspaceDataCacheProvider>
    );
    const view = render(workspace(true));
    await screen.findByRole("heading", { name: "Old track" });
    view.rerender(workspace(false));
    view.rerender(workspace(true));
    const oldCard = screen.getByRole("heading", { name: "Old track" }).closest("article")!;
    const player = oldCard.querySelector("audio");
    await waitFor(() => expect(api.loadMusicJobs).toHaveBeenCalledTimes(2));
    await act(async () => { finishHistory({ items: [newMusicJob, oldMusicJob], has_more: false, next_cursor: null }); });
    await screen.findByRole("heading", { name: "New track" });
    expect(screen.getByRole("heading", { name: "Old track" }).closest("article")?.querySelector("audio")).toBe(player);
    expect(vi.mocked(api.loadMusicJobResult).mock.calls.filter(([id]) => id === oldMusicJob.id)).toHaveLength(1);
    expect(vi.mocked(api.loadMusicJobResult).mock.calls.filter(([id]) => id === newMusicJob.id)).toHaveLength(1);
  });

  it("adds a completed new generation while retaining old tracks and their loaded results", async () => {
    const api = apiStub({
      loadMusicJobs: vi.fn().mockResolvedValue({ items: [oldMusicJob], has_more: false, next_cursor: null }),
      loadMusicJobResult: vi.fn(async id => audioResult(id === oldMusicJob.id ? oldMusicJob : newMusicJob, id === oldMusicJob.id ? "Old track" : "New track")),
      prepareMusicJob: vi.fn().mockResolvedValue({ balance: 100, can_afford: true, job: { ...newMusicJob, status: "prepared" } }),
      activateMusicJob: vi.fn().mockResolvedValue({ job: newMusicJob }),
      loadMusicJob: vi.fn().mockResolvedValue(newMusicJob),
    });
    render(<MusicWorkspaceController api={api} catalogLoader={loadedCatalog} />);
    await screen.findByRole("heading", { name: "Old track" });
    const player = screen.getByRole("heading", { name: "Old track" }).closest("article")?.querySelector("audio");
    fireEvent.change(screen.getByRole("textbox", { name: "Описание трека" }), { target: { value: "A new song" } });
    fireEvent.click(screen.getByRole("button", { name: "Создать песню" }));
    fireEvent.click(await screen.findByRole("button", { name: /Подтвердить/ }));
    await screen.findByRole("heading", { name: "New track" });
    await waitFor(() => expect(vi.mocked(api.loadMusicJobs).mock.calls.length).toBeGreaterThanOrEqual(2));
    expect(screen.getByRole("heading", { name: "Old track" }).closest("article")?.querySelector("audio")).toBe(player);
    expect(vi.mocked(api.loadMusicJobResult).mock.calls.filter(([id]) => id === oldMusicJob.id)).toHaveLength(1);
    expect(vi.mocked(api.loadMusicJobResult).mock.calls.filter(([id]) => id === newMusicJob.id)).toHaveLength(1);
  });
  it("selects a linked catalog model and responds to music-to-music navigation without starting jobs", async () => {
    const api = apiStub();
    const catalog: MusicModelCatalog = {
      ...enabledCatalog,
      models: [...enabledCatalog.models, { ...enabledCatalog.models[0], id: "lyria_3_5", name: "Lyria 3.5" }],
    };
    const loader = () => Promise.resolve(catalog);
    const { rerender } = render(<MusicWorkspaceController api={api} catalogLoader={loader} requestedModelId="lyria_3_5" />);
    await waitFor(() => expect(screen.getByRole("button", { name: "Модель генерации музыки" })).toHaveTextContent("Lyria 3.5"));
    rerender(<MusicWorkspaceController api={api} catalogLoader={loader} requestedModelId="suno_v6" />);
    await waitFor(() => expect(screen.getByRole("button", { name: "Модель генерации музыки" })).toHaveTextContent("Suno V6"));
    rerender(<MusicWorkspaceController api={api} catalogLoader={loader} requestedModelId="not-a-model" />);
    await waitFor(() => expect(screen.getByRole("button", { name: "Модель генерации музыки" })).toHaveTextContent("Suno V6"));
    expect(api.prepareMusicJob).not.toHaveBeenCalled();
    expect(api.activateMusicJob).not.toHaveBeenCalled();
  });

  it("keeps pending catalog candidates visible but not runnable", async () => {
    const api = apiStub();
    render(<MusicWorkspaceController api={api} catalogLoader={() => Promise.resolve(pendingCatalog)} />);

    expect(await screen.findByText("Эта модель пока недоступна. Можно подготовить описание и настройки.")).toBeVisible();
    fireEvent.change(screen.getByRole("textbox", { name: "Описание трека" }), {
      target: { value: "make a chorus" },
    });

    expect(screen.getByRole("button", { name: "Создать песню" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Создать песню" }));
    expect(api.prepareMusicJob).not.toHaveBeenCalled();
  });

  it("shows the exact prepare quote and sends only provider-neutral options", async () => {
    const api = apiStub({
      prepareMusicJob: vi.fn().mockResolvedValue({
        balance: 100,
        can_afford: true,
        job: {
          action: "generate",
          cost_estimate: 37,
          created_at: "2026-09-16T10:00:00Z",
          id: "33333333-3333-4333-8333-333333333333",
          model_id: "suno_v6",
          status: "prepared",
        },
      }),
    });
    const keys = ["11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"];

    render(
      <MusicWorkspaceController
        api={api}
        catalogLoader={() => Promise.resolve(enabledCatalog)}
        uuidFactory={() => keys.shift() ?? "44444444-4444-4444-8444-444444444444"}
      />,
    );

    fireEvent.change(await screen.findByRole("textbox", { name: "Описание трека" }), {
      target: { value: "ambient pop" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Создать песню" }));

    await waitFor(() => expect(api.prepareMusicJob).toHaveBeenCalledTimes(1));
    expect(api.prepareMusicJob).toHaveBeenCalledWith({
      audio_artifact_ids: [],
      model_id: "suno_v6",
      music: expect.objectContaining({
        action: "generate",
        audio_format: "mp3",
        prompt: "ambient pop",
      }),
      sources: [],
    }, "11111111-1111-4111-8111-111111111111");
    expect(screen.getByLabelText("Стоимость: 37 звёзд")).toBeInTheDocument();
  });

  it("relocalizes a visible confirmation without reloading the catalog or preparing again", async () => {
    const api = apiStub({
      prepareMusicJob: vi.fn().mockResolvedValue({
        balance: 100,
        can_afford: true,
        job: {
          action: "generate",
          cost_estimate: 37,
          created_at: "2026-09-16T10:00:00Z",
          id: "33333333-3333-4333-8333-333333333333",
          model_id: "suno_v6",
          status: "prepared",
        },
      }),
    });
    const catalogLoader = vi.fn().mockResolvedValue(enabledCatalog);
    const keys = ["11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"];
    const view = (locale: "ru" | "en") => (
      <LocaleProvider locale={locale}>
        <MusicWorkspaceController
          api={api}
          catalogLoader={catalogLoader}
          uuidFactory={() => keys.shift() ?? "44444444-4444-4444-8444-444444444444"}
        />
      </LocaleProvider>
    );
    const { rerender } = render(view("ru"));

    fireEvent.change(await screen.findByRole("textbox", { name: "Описание трека" }), {
      target: { value: "ambient pop" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Создать песню" }));
    await screen.findByRole("heading", { name: "Подтверждение запуска" });

    rerender(view("en"));

    expect(await screen.findByRole("heading", { name: "Run confirmation" })).toBeVisible();
    expect(screen.getByRole("textbox", { name: "Track description" })).toHaveValue("ambient pop");
    expect(api.prepareMusicJob).toHaveBeenCalledTimes(1);
    expect(catalogLoader).toHaveBeenCalledTimes(1);
  });

  it("relocalizes a visible prepare error without retrying prepare", async () => {
    const api = apiStub({
      prepareMusicJob: vi.fn().mockRejectedValue(new MusicApiError("invalid music request", 400)),
    });
    const catalogLoader = vi.fn().mockResolvedValue(enabledCatalog);
    const view = (locale: "ru" | "en") => (
      <LocaleProvider locale={locale}>
        <MusicWorkspaceController api={api} catalogLoader={catalogLoader} />
      </LocaleProvider>
    );
    const { rerender } = render(view("ru"));

    fireEvent.change(await screen.findByRole("textbox", { name: "Описание трека" }), {
      target: { value: "ambient pop" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Создать песню" }));
    await screen.findByRole("alert", { name: "" });
    expect(screen.getByRole("alert")).toHaveTextContent("Сервер отклонил параметры музыкального запуска.");

    rerender(view("en"));

    expect(screen.getByRole("alert")).toHaveTextContent("The server rejected the music run parameters.");
    expect(screen.getByRole("textbox", { name: "Track description" })).toHaveValue("ambient pop");
    expect(api.prepareMusicJob).toHaveBeenCalledTimes(1);
    expect(catalogLoader).toHaveBeenCalledTimes(1);
  });

  it("renders text results and artifact downloads loaded from completed history", async () => {
    const api = apiStub({
      loadMusicJobResult: vi.fn().mockResolvedValue({
        artifacts: [{
          format: "txt",
          id: "44444444-4444-4444-8444-444444444444",
          kind: "lyrics",
          url: "/web/v1/music-artifacts/44444444-4444-4444-8444-444444444444",
        }],
        bpm: { average: 124, maximum: 128, minimum: 120 },
        job_id: "33333333-3333-4333-8333-333333333333",
        lyrics: [{ tags: "chorus", text: "line one\nline two", title: "Generated lyric" }],
        tags: "synth pop",
        tracks: [],
      }),
      loadMusicJobs: vi.fn().mockResolvedValue({
        has_more: false,
        items: [{
          action: "lyrics",
          cost_estimate: 3,
          created_at: "2026-09-16T10:00:00Z",
          id: "33333333-3333-4333-8333-333333333333",
          model_id: "suno_v6",
          status: "succeeded",
        }],
        next_cursor: null,
      }),
    });

    render(<MusicWorkspaceController api={api} catalogLoader={() => Promise.resolve(enabledCatalog)} />);

    await waitFor(() => expect(api.loadMusicJobResult).toHaveBeenCalledWith("33333333-3333-4333-8333-333333333333", expect.any(AbortSignal)));
    expect(await screen.findByText((_, element) => element?.textContent === "line one\nline two")).toBeVisible();
    expect(screen.getByText("Теги: synth pop")).toBeVisible();
    expect(screen.getByText("BPM: 124 · диапазон 120–128")).toBeVisible();
    expect(screen.getByRole("link", { name: "Скачать файл TXT" })).toHaveAttribute(
      "href",
      "/web/v1/music-artifacts/44444444-4444-4444-8444-444444444444",
    );
  });

  it("continues polling through result_ready and loads the result only after succeeded", async () => {
    const job = {
      action: "generate" as const,
      cost_estimate: 37,
      created_at: "2026-09-16T10:00:00Z",
      id: "33333333-3333-4333-8333-333333333333",
      model_id: "suno_v6",
    };
    const api = apiStub({
      loadMusicJob: vi.fn()
        .mockResolvedValueOnce({ ...job, status: "result_ready" })
        .mockResolvedValueOnce({ ...job, status: "succeeded" }),
      loadMusicJobResult: vi.fn().mockResolvedValue({
        artifacts: [],
        job_id: job.id,
        lyrics: [{ text: "ready lyric" }],
        tracks: [],
      }),
      loadMusicJobs: vi.fn().mockResolvedValue({
        has_more: false,
        items: [{ ...job, status: "result_ready" }],
        next_cursor: null,
      }),
    });

    render(<MusicWorkspaceController api={api} catalogLoader={() => Promise.resolve(enabledCatalog)} pollIntervalMs={1} />);

    await waitFor(() => expect(api.loadMusicJob).toHaveBeenCalledTimes(2));
    expect(api.loadMusicJobResult).toHaveBeenCalledWith(job.id, expect.any(AbortSignal));
    expect(await screen.findByText("ready lyric")).toBeVisible();
  });

  it("loads older history pages so reusable persona and model assets stay selectable", async () => {
    const personaJobID = "44444444-4444-4444-8444-444444444444";
    const modelJobID = "55555555-5555-4555-8555-555555555555";
    const api = apiStub({
      loadMusicJobResult: vi.fn()
        .mockImplementation((jobID: string) => Promise.resolve({
          artifacts: [],
          job_id: jobID,
          model: jobID === modelJobID ? { job_id: modelJobID, name: "Lead model" } : undefined,
          persona: jobID === personaJobID ? { job_id: personaJobID, name: "Bright persona" } : undefined,
          tracks: [],
        })),
      loadMusicJobs: vi.fn()
        .mockResolvedValueOnce({
          has_more: true,
          items: [],
          next_cursor: "older-page",
        })
        .mockResolvedValueOnce({
          has_more: false,
          items: [
            {
              action: "persona",
              cost_estimate: 5,
              created_at: "2026-09-16T09:00:00Z",
              id: personaJobID,
              model_id: "suno_v6",
              status: "succeeded",
            },
            {
              action: "create_model",
              cost_estimate: 5,
              created_at: "2026-09-16T08:00:00Z",
              id: modelJobID,
              model_id: "suno_v6",
              status: "succeeded",
            },
          ],
          next_cursor: null,
        }),
    });

    render(<MusicWorkspaceController api={api} catalogLoader={() => Promise.resolve(enabledCatalog)} />);
    fireEvent.click(await screen.findByRole("button", { name: "Загрузить ещё" }));
    fireEvent.click(screen.getByRole("button", { name: "Настройки трека" }));

    await waitFor(() => expect(api.loadMusicJobs).toHaveBeenLastCalledWith(8, "older-page", expect.any(AbortSignal)));
    expect(await screen.findByRole("option", { name: "Bright persona" })).toBeInTheDocument();
    expect(await screen.findByRole("option", { name: "Lead model" })).toBeInTheDocument();
  });
});
