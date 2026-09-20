import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

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

describe("MusicWorkspaceController", () => {
  it("keeps pending catalog candidates visible but not runnable", async () => {
    const api = apiStub();
    render(<MusicWorkspaceController api={api} catalogLoader={() => Promise.resolve(pendingCatalog)} />);

    expect(await screen.findAllByText("Модель ждёт отдельной проверки")).not.toHaveLength(0);
    fireEvent.change(screen.getByRole("textbox", { name: "Описание трека" }), {
      target: { value: "make a chorus" },
    });

    expect(screen.getByRole("button", { name: "Сгенерировать" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Сгенерировать" }));
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
    fireEvent.click(screen.getByRole("button", { name: "Сгенерировать" }));

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

    await waitFor(() => expect(api.loadMusicJobResult).toHaveBeenCalledWith("33333333-3333-4333-8333-333333333333"));
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
    expect(api.loadMusicJobResult).toHaveBeenCalledWith(job.id);
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

    await waitFor(() => expect(api.loadMusicJobs).toHaveBeenLastCalledWith(8, "older-page"));
    expect(await screen.findByRole("option", { name: "Bright persona" })).toBeInTheDocument();
    expect(await screen.findByRole("option", { name: "Lead model" })).toBeInTheDocument();
  });
});
