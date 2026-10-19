"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { LoadingIndicator, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { Button } from "@/components/ui/Button/Button";
import { MusicTrackCard } from "@/features/music/MusicTrackCard/MusicTrackCard";
import { musicApi, musicTracksFromResult, type MusicJob, type MusicJobResult } from "@/features/music/music-api";
import { loadMusicModelCatalog, type MusicModelCatalog } from "@/features/music/music-model-catalog";
import { musicContinuationHref } from "@/features/music/music-navigation";
import { createMusicResultCache, createMusicResultReader } from "@/features/music/music-result-cache";
import { useOptionalWorkspaceDataCache } from "@/features/workspace/WorkspaceDataCache/WorkspaceDataCache";
import { useDictionary, useMessages } from "@/i18n/LocaleProvider";
import styles from "./MusicFiles.module.css";

type Entry = { job: MusicJob; result: MusicJobResult };
type MusicFilesAPI = Pick<typeof musicApi, "loadMusicJobs" | "loadMusicJobResult">;

function mergeEntries(current: Entry[], incoming: Entry[]) {
  const entries = new Map(current.map(entry => [entry.job.id, entry]));
  for (const entry of incoming) entries.set(entry.job.id, entry);
  return [...entries.values()].sort((a, b) => Date.parse(b.job.created_at) - Date.parse(a.job.created_at));
}

export function MusicFiles({ visible, showEmpty = true, onHasFilesChange, api = musicApi, catalogLoader = loadMusicModelCatalog }: Readonly<{
  visible: boolean;
  showEmpty?: boolean;
  onHasFilesChange?: (hasFiles: boolean) => void;
  api?: MusicFilesAPI;
  catalogLoader?: () => Promise<MusicModelCatalog>;
}>) {
  const t = useDictionary();
  const msg = useMessages();
  const workspace = useOptionalWorkspaceDataCache();
  const [cache] = useState(() => workspace?.musicResults ?? createMusicResultCache());
  const [entries, setEntries] = useState<Entry[]>(() => cache.getEntries());
  const [catalog, setCatalog] = useState<MusicModelCatalog | null>(null);
  const [hasLoaded, setHasLoaded] = useState(entries.length > 0);
  const [loading, setLoading] = useState(false);
  const [failed, setFailed] = useState(false);
  const [retryCursor, setRetryCursor] = useState<string | undefined>();
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const request = useRef<AbortController | null>(null);
  const started = useRef(false);

  const loadPage = useCallback(async (cursor?: string) => {
    request.current?.abort();
    const controller = new AbortController();
    request.current = controller;
    setLoading(true);
    setFailed(false);
    try {
      const page = await api.loadMusicJobs(12, cursor, controller.signal);
      if (controller.signal.aborted) return;
      const reader = createMusicResultReader(cache, (id, signal) => api.loadMusicJobResult(id, signal), controller.signal);
      const jobs = page.items.filter(job => job.status === "succeeded");
      let partialFailure = false;
      // Bound history hydration; already cached results issue no new reads.
      for (let index = 0; index < jobs.length; index += 2) {
        const batch = jobs.slice(index, index + 2);
        const loaded = await Promise.allSettled(batch.map(job => reader(job)));
        if (controller.signal.aborted) return;
        const ready: Entry[] = [];
        loaded.forEach((item, offset) => {
          if (item.status === "fulfilled") ready.push({ job: batch[offset], result: item.value });
          else partialFailure = true;
        });
        setEntries(current => mergeEntries(current, ready));
      }
      setNextCursor(page.next_cursor);
      setHasLoaded(true);
      setFailed(partialFailure);
      setRetryCursor(cursor);
    } catch {
      if (!controller.signal.aborted) { setFailed(true); setRetryCursor(cursor); }
    } finally {
      if (!controller.signal.aborted) setLoading(false);
    }
  }, [api, cache]);

  useEffect(() => () => { request.current?.abort(); started.current = false; }, []);
  useEffect(() => {
    if (!visible || started.current) return;
    started.current = true;
    void loadPage();
  }, [loadPage, visible]);
  useEffect(() => {
    if (!visible) return;
    let active = true;
    void catalogLoader().then(value => { if (active) setCatalog(value); }).catch(() => undefined);
    return () => { active = false; };
  }, [catalogLoader, visible]);
  useEffect(() => {
    if (hasLoaded) onHasFilesChange?.(entries.some(entry => entry.result.tracks.length > 0));
  }, [entries, hasLoaded, onHasFilesChange]);

  if (!visible) return null;
  const tracks = entries.flatMap(entry => musicTracksFromResult(entry.result, entry.job));

  return <section aria-label={msg("music.files.heading")} aria-busy={loading || undefined} className={styles.root}>
    <h2>{msg("music.files.heading")}</h2>
    {loading && tracks.length === 0 ? <LoadingIndicator label={t.files.loading} /> : null}
    <div className={styles.tracks}>
      {tracks.map(track => {
        const model = catalog?.models.find(model => model.id === track.modelId);
        const canContinue = (track.durationSec ?? 0) > 1 && model?.enabled && model.operations.some(op => op.id === "extend" && op.enabled);
        return <MusicTrackCard key={track.id} track={track} continueHref={canContinue ? musicContinuationHref(track) : undefined} />;
      })}
    </div>
    {failed ? <StateNotice kind="error" action={{ label: t.files.retry, disabled: loading, onClick: () => void loadPage(retryCursor) }}>{msg("music.files.error")}</StateNotice> : null}
    {hasLoaded && tracks.length === 0 && !failed && showEmpty ? <StateNotice kind="empty">{msg("music.files.empty")}</StateNotice> : null}
    {nextCursor !== null ? <Button variant="outline" disabled={loading} onClick={() => void loadPage(nextCursor)}>{loading ? t.files.loadingMore : t.files.loadMore}</Button> : null}
  </section>;
}
