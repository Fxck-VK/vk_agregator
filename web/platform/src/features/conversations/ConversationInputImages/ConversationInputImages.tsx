"use client";

import { MediaState } from "@/components/ui/AsyncState/AsyncState";
import { RetryAction } from "@/components/ui/AsyncState/RetryAction";
import Image from "next/image";
import { useCallback, useEffect, useState } from "react";
import { AttachmentPreviewDialog } from "@/components/media/AttachmentPreviewDialog/AttachmentPreviewDialog";
import { useDictionary, useMessages } from "@/i18n/LocaleProvider";
import { webBrowserFetch } from "@/lib/web-api/browser";
import styles from "./ConversationInputImages.module.css";

export function ConversationInputImages({ ids }: { ids?: readonly string[] }) {
  const msg = useMessages();
  const [urls, setUrls] = useState<Record<string, string>>({});
  const [preview, setPreview] = useState<{ id: string; trigger: HTMLButtonElement } | null>(null);
  const updateURL = useCallback((id: string, url: string | null) => {
    setUrls(current => {
      if (current[id] === (url ?? undefined)) return current;
      const next = { ...current };
      if (url) next[id] = url;
      else delete next[id];
      return next;
    });
  }, []);
  if (!ids?.length) return null;
  const items = ids.flatMap(id => urls[id] ? [{ id, src: urls[id], alt: msg("conversationHistory.attachedImage") }] : []);
  const selectedIndex = items.findIndex(item => item.id === preview?.id);
  return <>
    <div className={styles.images}>{ids.map(id => <InputImage key={id} id={id} onReady={updateURL}
      onOpen={trigger => setPreview({ id, trigger })} />)}</div>
    {preview && selectedIndex >= 0 ? <AttachmentPreviewDialog
      items={items}
      selectedIndex={selectedIndex}
      onSelect={index => setPreview({ ...preview, id: items[index].id })}
      onClose={() => setPreview(null)}
      returnFocusTo={preview.trigger}
    /> : null}
  </>;
}

function InputImage({ id, onReady, onOpen }: {
  id: string;
  onReady: (id: string, url: string | null) => void;
  onOpen: (trigger: HTMLButtonElement) => void;
}) {
  const msg = useMessages();
  const t = useDictionary();
  const [attempt, setAttempt] = useState(0);
  const [result, setResult] = useState<{ attempt: number; url?: string; failed?: boolean } | null>(null);
  const current = result?.attempt === attempt ? result : null;
  useEffect(() => {
    const controller = new AbortController();
    let objectURL: string | undefined;
    const load = async () => {
      try {
        const response = await webBrowserFetch(`/web/v1/input-artifacts/${id}`, { signal: controller.signal });
        if (!response.ok) throw new Error("Image unavailable");
        const blob = await response.blob();
        controller.signal.throwIfAborted();
        if (!["image/png", "image/jpeg", "image/webp"].includes(blob.type)) throw new Error("Unsupported image");
        objectURL = URL.createObjectURL(blob);
        setResult({ attempt, url: objectURL });
        onReady(id, objectURL);
      } catch {
        if (!controller.signal.aborted) setResult({ attempt, failed: true });
      }
    };
    void load();
    return () => {
      controller.abort();
      onReady(id, null);
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [id, attempt, onReady]);
  return <div className={styles.image} data-ui="conversation-input-image">
    {current?.url && !current.failed ? (
      <button type="button" className={styles.openPreview} aria-haspopup="dialog"
        aria-label={`${t.files.previewDialogLabel}: ${msg("conversationHistory.attachedImage")}`}
        onClick={event => onOpen(event.currentTarget)}>
        <Image unoptimized src={current.url} alt={msg("conversationHistory.attachedImage")} width={160} height={160}
          onError={() => { setResult({ attempt, failed: true }); onReady(id, null); }} />
      </button>
    ) : current?.failed ? (
      <RetryAction iconOnly label={msg("chatComposer.uploadFailedRetry")} aria-label={msg("conversationInputImages.retry")} onClick={() => setAttempt(value => value + 1)} />
    ) : <MediaState label={msg("conversationInputImages.loading")} />}
  </div>;
}
