import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { MusicFiles } from "./MusicFiles";
import type { MusicJob, MusicJobResult } from "@/features/music/music-api";
import { WorkspaceDataCacheProvider, useWorkspaceDataCache } from "@/features/workspace/WorkspaceDataCache/WorkspaceDataCache";

const oldJob: MusicJob = { id: "11111111-1111-4111-8111-111111111111", model_id: "suno_v6", action: "generate", status: "succeeded", cost_estimate: 30, created_at: "2026-10-19T07:00:00Z" };
const nextJob: MusicJob = { ...oldJob, id: "22222222-2222-4222-8222-222222222222", created_at: "2026-10-19T06:00:00Z" };
const result = (job: MusicJob, title: string): MusicJobResult => ({ job_id: job.id, artifacts: [], tracks: [{ job_id: job.id, audio_index: 1, title, duration_sec: 120, audio_url: `/web/v1/music-artifacts/${job.id}` }] });
const catalogLoader = vi.fn(async () => ({ defaultModelId: "suno_v6" as const, models: [] }));

describe("MusicFiles", () => {
  it("reuses the account music cache, carries the chosen continuation time and clears on account change", async () => {
    function Seed() {
      const cache = useWorkspaceDataCache();
      cache.musicResults.remember(oldJob, result(oldJob, "Cached song"));
      return null;
    }
    const api = { loadMusicJobs: vi.fn().mockResolvedValue({ items: [oldJob], has_more: false, next_cursor: null }), loadMusicJobResult: vi.fn() };
    const loader = async () => ({ defaultModelId: "suno_v6" as const, models: [{ id: "suno_v6" as const, name: "Suno V6", enabled: true, operations: [{ id: "extend" as const, enabled: true }] }] });
    const view = render(<WorkspaceDataCacheProvider key="owner-one"><Seed /><MusicFiles visible api={api} catalogLoader={loader} /></WorkspaceDataCacheProvider>);
    expect(screen.getByRole("heading", { name: "Cached song" })).toBeVisible();
    await waitFor(() => expect(api.loadMusicJobs).toHaveBeenCalledOnce());
    expect(api.loadMusicJobResult).not.toHaveBeenCalled();
    fireEvent.change(screen.getByRole("slider", { name: "Перемотка: Cached song" }), { target: { value: "45" } });
    fireEvent.click(screen.getByRole("button", { name: "Действия: Cached song" }));
    const link = await screen.findByRole("menuitem", { name: "Продолжить трек" });
    expect(link).toHaveAttribute("href", `/ru/app/music?model=suno_v6&action=extend&source_job_id=${oldJob.id}&audio_index=1&continue_at=45`);
    const otherAPI = { loadMusicJobs: vi.fn().mockResolvedValue({ items: [], has_more: false, next_cursor: null }), loadMusicJobResult: vi.fn() };
    view.rerender(<WorkspaceDataCacheProvider key="owner-two"><MusicFiles visible api={otherAPI} catalogLoader={loader} /></WorkspaceDataCacheProvider>);
    expect(screen.queryByRole("heading", { name: "Cached song" })).not.toBeInTheDocument();
    await screen.findByText("Здесь появятся ваши готовые треки.");
  });

  it("loads only when visible and adds pages without remounting existing audio", async () => {
    const api = {
      loadMusicJobs: vi.fn().mockResolvedValueOnce({ items: [oldJob], has_more: true, next_cursor: "next" }).mockResolvedValueOnce({ items: [oldJob, nextJob], has_more: false, next_cursor: null }),
      loadMusicJobResult: vi.fn(async id => result(id === oldJob.id ? oldJob : nextJob, id === oldJob.id ? "First song" : "Second song")),
    };
    const { rerender, container } = render(<MusicFiles visible={false} api={api} catalogLoader={catalogLoader} />);
    expect(api.loadMusicJobs).not.toHaveBeenCalled();
    rerender(<MusicFiles visible api={api} catalogLoader={catalogLoader} />);
    await screen.findByRole("heading", { name: "First song" });
    const player = container.querySelector("audio");
    fireEvent.click(screen.getByRole("button", { name: "Показать ещё" }));
    await screen.findByRole("heading", { name: "Second song" });
    expect(container.querySelector("audio")).toBe(player);
    expect(api.loadMusicJobResult).toHaveBeenCalledTimes(2);
    expect(screen.getAllByRole("heading", { name: "First song" })).toHaveLength(1);
  });

  it("keeps successful cards on a failed page and retries the read without creating a job", async () => {
    const api = {
      loadMusicJobs: vi.fn().mockResolvedValueOnce({ items: [oldJob], has_more: true, next_cursor: "next" }).mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce({ items: [nextJob], has_more: false, next_cursor: null }),
      loadMusicJobResult: vi.fn(async id => result(id === oldJob.id ? oldJob : nextJob, id === oldJob.id ? "First song" : "Second song")),
    };
    render(<MusicFiles visible api={api} catalogLoader={catalogLoader} />);
    await screen.findByRole("heading", { name: "First song" });
    fireEvent.click(screen.getByRole("button", { name: "Показать ещё" }));
    await screen.findByText("Не удалось загрузить часть треков. Попробуйте ещё раз.");
    expect(screen.getByRole("heading", { name: "First song" })).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    await screen.findByRole("heading", { name: "Second song" });
    expect(api.loadMusicJobResult).toHaveBeenCalledTimes(2);
  });

  it("aborts pending history when the account workspace unmounts", async () => {
    let requestSignal: AbortSignal | undefined;
    const api = { loadMusicJobs: vi.fn((_limit, _cursor, signal) => { requestSignal = signal; return new Promise<never>(() => {}); }), loadMusicJobResult: vi.fn() };
    const { unmount } = render(<MusicFiles visible api={api} catalogLoader={catalogLoader} />);
    await waitFor(() => expect(api.loadMusicJobs).toHaveBeenCalledOnce());
    unmount();
    expect(requestSignal?.aborted).toBe(true);
  });
});
