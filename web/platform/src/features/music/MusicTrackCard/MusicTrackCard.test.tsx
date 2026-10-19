import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MusicTrackCard } from "./MusicTrackCard";
import type { MusicTrack } from "../MusicWorkspace";

const track: MusicTrack = {
  id: "track-a", sourceJobId: "11111111-1111-4111-8111-111111111111", originalIndex: 0,
  artifactPlaybackUrl: "/web/v1/music-artifacts/11111111-1111-4111-8111-111111111111",
  title: "Forest song", durationSec: 120,
  downloads: [{ kind: "audio", format: "mp3", label: "MP3", url: "/web/v1/music-artifacts/11111111-1111-4111-8111-111111111111" }],
};

afterEach(() => vi.restoreAllMocks());

describe("MusicTrackCard", () => {
  it("shows a compact player and keeps downloads inside the action menu", () => {
    vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {});
    const onContinue = vi.fn();
    render(<MusicTrackCard track={track} onContinue={onContinue} />);
    expect(screen.getByRole("button", { name: "Воспроизвести: Forest song" })).toBeVisible();
    expect(screen.getByRole("slider", { name: "Перемотка: Forest song" })).toHaveAttribute("max", "120");
    expect(screen.queryByRole("link", { name: "Скачать аудио MP3" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Действия: Forest song" }));
    const menu = screen.getByRole("menu");
    expect(within(menu).getByRole("menuitem", { name: "Скачать аудио MP3" })).toHaveAttribute("download");
    fireEvent.click(within(menu).getByRole("menuitem", { name: "Продолжить трек" }));
    expect(onContinue).toHaveBeenCalledOnce();
  });

  it("plays only after an explicit click and pauses the previous card", async () => {
    const play = vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue();
    const pause = vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {});
    const { container, rerender } = render(<><MusicTrackCard track={track} /><MusicTrackCard track={{ ...track, id: "track-b", title: "Second song" }} /></>);
    expect(play).not.toHaveBeenCalled();
    const audios = container.querySelectorAll("audio");
    fireEvent.click(screen.getByRole("button", { name: "Воспроизвести: Forest song" }));
    await waitFor(() => expect(play).toHaveBeenCalledOnce());
    fireEvent.play(audios[0]);
    fireEvent.change(screen.getByRole("slider", { name: "Перемотка: Forest song" }), { target: { value: "45" } });
    expect(audios[0].currentTime).toBe(45);
    fireEvent.play(audios[1]);
    expect(pause).toHaveBeenCalled();
    rerender(<><MusicTrackCard track={track} /><MusicTrackCard track={{ ...track, id: "track-b", title: "Second song" }} /></>);
    expect(container.querySelector("audio")).toBe(audios[0]);
    expect(play).toHaveBeenCalledOnce();
  });

  it("does not expose a continuation or unsafe media when they are unavailable", () => {
    vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {});
    const { container } = render(<MusicTrackCard track={{ ...track, artifactPlaybackUrl: "https://foreign.example/audio", downloads: [] }} />);
    expect(container.querySelector("audio")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Действия: Forest song" }));
    expect(screen.queryByRole("menuitem", { name: "Продолжить трек" })).not.toBeInTheDocument();
    expect(container.querySelector('a[href^="https://foreign"]')).toBeNull();
  });
});
