"use client";

/* eslint-disable @next/next/no-img-element */

import { useEffect, useId, useRef, useState } from "react";
import { PopoverPanel } from "@/components/ui/PopoverPanel/PopoverPanel";
import Link from "@/i18n/Link";
import { useMessages } from "@/i18n/LocaleProvider";
import type { MusicTrack } from "../MusicWorkspace";
import { musicArtifactLabel, normalizeMusicArtifactURL } from "../music-api";
import styles from "./MusicTrackCard.module.css";
import { ArtifactDeleteButton } from "@/features/files/ArtifactDeleteButton";
import { artifactIDFromPath, useArtifactDeletion } from "@/features/files/artifact-deletion";

let playingAudio: HTMLAudioElement | null = null;

export function musicTime(seconds: number) {
  const value = Number.isFinite(seconds) ? Math.max(0, Math.floor(seconds)) : 0;
  return `${Math.floor(value / 60)}:${String(value % 60).padStart(2, "0")}`;
}

export function MusicTrackCard({ track, onContinue, continueHref, selected, onSelect, selectLabel, selectionDisabled, showDeleteControl = false }: Readonly<{
  track: MusicTrack;
  onContinue?: (continueAtSec: number) => void;
  continueHref?: string;
  selected?: boolean;
  onSelect?: () => void;
  selectLabel?: string;
  selectionDisabled?: boolean;
  showDeleteControl?: boolean;
}>) {
  const msg = useMessages();
  const deletion = useArtifactDeletion();
  const audioRef = useRef<HTMLAudioElement>(null);
  const menuRef = useRef<HTMLButtonElement>(null);
  const menuID = useId();
  const [menuOpen, setMenuOpen] = useState(false);
  const [lyricsOpen, setLyricsOpen] = useState(false);
  const [playing, setPlaying] = useState(false);
  const [position, setPosition] = useState(0);
  const [positionChosen, setPositionChosen] = useState(false);
  const [mediaDuration, setMediaDuration] = useState(0);
  const [failed, setFailed] = useState(false);
  const title = track.title || msg(track.sourceAction === "upload" ? "music.tools.imported" : "music.track.titleFallback");
  const sourceID = artifactIDFromPath(track.artifactPlaybackUrl);
  const sourceDeleted = sourceID ? deletion.isDeleted(sourceID) : false;
  const source = sourceDeleted ? undefined : normalizeMusicArtifactURL(track.artifactPlaybackUrl);
  const coverID = artifactIDFromPath(track.coverImageUrl);
  const cover = coverID && deletion.isDeleted(coverID) ? undefined : normalizeMusicArtifactURL(track.coverImageUrl);
  const duration = mediaDuration || track.durationSec || 0;
  const continueAt = Math.max(1, Math.min(Math.floor(positionChosen ? position : duration - 1), Math.floor(duration) - 1));
  const downloads = (track.downloads ?? []).filter(a => normalizeMusicArtifactURL(a.url) && !deletion.isDeleted(artifactIDFromPath(a.url) ?? ""));

  useEffect(() => {
    const audio = audioRef.current;
    return () => {
      if (audio && !audio.paused) audio.pause();
      if (playingAudio === audio) playingAudio = null;
    };
  }, [source]);

  const togglePlayback = () => {
    const audio = audioRef.current;
    if (!audio) return;
    if (!audio.paused) { audio.pause(); return; }
    setFailed(false);
    void audio.play().catch(() => { setPlaying(false); setFailed(true); });
  };

  if (sourceDeleted) return null;
  return (
    <article className={styles.card} data-selected={selected || undefined}>
      {source ? <audio hidden preload="none" ref={audioRef} src={source}
        onPlay={event => {
          if (playingAudio && playingAudio !== event.currentTarget) playingAudio.pause();
          playingAudio = event.currentTarget;
          setPlaying(true);
        }}
        onPause={() => setPlaying(false)} onEnded={() => setPlaying(false)}
        onError={() => { setFailed(true); setPlaying(false); }}
        onLoadedMetadata={event => setMediaDuration(Number.isFinite(event.currentTarget.duration) ? event.currentTarget.duration : 0)}
        onTimeUpdate={event => { setPosition(event.currentTarget.currentTime); setPositionChosen(true); }}
      /> : null}
      <button aria-label={msg(playing ? "music.card.pause" : "music.card.play", { title })}
        aria-pressed={playing} className={styles.play} disabled={!source} onClick={togglePlayback} type="button">
        {cover ? <img alt="" src={cover} loading="lazy" /> : <span className={styles.note} aria-hidden="true">♪</span>}
        <span className={styles.playIcon} aria-hidden="true">{playing ? "Ⅱ" : "▶"}</span>
      </button>
      <div className={styles.body}>
        <h3 title={title}>{title}</h3>
        <div className={styles.progress}>
          <input aria-label={msg("music.card.seek", { title })} disabled={!source || duration <= 0}
            max={duration || 1} min={0} step={0.1} type="range" value={Math.min(position, duration || 1)}
            onChange={event => {
              const seconds = Number(event.target.value);
              if (audioRef.current) audioRef.current.currentTime = seconds;
              setPosition(seconds);
              setPositionChosen(true);
            }} />
          <span className={styles.time}>{musicTime(position)} / {musicTime(duration)}</span>
        </div>
        {failed ? <p className={styles.error} role="status">{msg("music.card.playbackError")}</p> : null}
      </div>
      <div className={styles.actions}>
        {showDeleteControl && sourceID ? <ArtifactDeleteButton artifactID={sourceID} className={styles.select} relatedArtifactIDs={downloads.flatMap(item => { const id = artifactIDFromPath(item.url); return id ? [id] : []; })} /> : null}
        {onSelect ? <button aria-label={selectLabel} aria-pressed={selected} className={styles.select} disabled={selectionDisabled} onClick={onSelect} type="button">{selected ? "✓" : "+"}</button> : null}
        <button aria-controls={menuID} aria-expanded={menuOpen} aria-haspopup="menu" aria-label={msg("music.card.actions", { title })}
          className={styles.menuButton} onClick={() => setMenuOpen(!menuOpen)} ref={menuRef} type="button">⋯</button>
      </div>
      <PopoverPanel align="end" anchorRef={menuRef} id={menuID} isOpen={menuOpen} itemVariant="action"
        label={msg("music.card.actions", { title })} onClose={() => setMenuOpen(false)} role="menu" width={248}>
        <div className={styles.menu}>
          {downloads.map(a => <a className={styles.menuItem} download href={a.url} key={a.url} role="menuitem">
            {musicArtifactLabel(a.kind, a.format, msg)}
          </a>)}
          {onContinue ? <button className={styles.menuItem} onClick={() => { setMenuOpen(false); onContinue(continueAt); }} role="menuitem" type="button">{msg("music.card.continue")}</button> : null}
          {continueHref ? <Link className={styles.menuItem} href={`${continueHref}&continue_at=${continueAt}`} role="menuitem">{msg("music.card.continue")}</Link> : null}
          {track.lyrics ? <button className={styles.menuItem} onClick={() => { setMenuOpen(false); setLyricsOpen(!lyricsOpen); }} role="menuitem" type="button">{msg("music.card.lyrics")}</button> : null}
          {downloads.length === 0 && !onContinue && !continueHref && !track.lyrics ? <span className={styles.empty}>{msg("music.card.noActions")}</span> : null}
        </div>
      </PopoverPanel>
      {lyricsOpen && track.lyrics ? <pre className={styles.lyrics}>{track.lyrics}</pre> : null}
    </article>
  );
}
