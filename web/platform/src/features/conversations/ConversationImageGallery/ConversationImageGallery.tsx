"use client";

import { MediaVideo } from "@/components/media/MediaVideo/MediaVideo";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useMessages, useDictionary } from "@/i18n/LocaleProvider";

import { createContext, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { AssistantMessageContent } from "@/components/chat/AssistantMessageContent/AssistantMessageContent";
import { FileCard } from "@/features/files/FileCard/FileCard";
import { FilePreviewDialog } from "@/features/files/FilePreviewDialog/FilePreviewDialog";
import { ImageGenerationGrid } from "@/features/image-generation/ImageGenerationGrid/ImageGenerationGrid";
import type { ConversationImage, ConversationMessage } from "@/lib/web-api/contracts";

import { collectConversationImages, loadEarlierConversationImages } from "./conversation-images";
import styles from "./ConversationImageGallery.module.css";

type GalleryContext = {
  openPreview: (image: ConversationImage, trigger: HTMLButtonElement) => void;
  pendingArtifactID: string | null;
  failedArtifactID: string | null;
};

const GalleryContext = createContext<GalleryContext | null>(null);
const noAction = () => {};

export function ConversationImageGallery({ children, conversationID, hasMoreBefore, messages }: Readonly<{
  children: ReactNode;
  conversationID: string;
  hasMoreBefore: boolean;
  messages: readonly ConversationMessage[];
}>) {
  const [earlierMessages, setEarlierMessages] = useState<ConversationMessage[]>([]);
  const [selectedArtifactID, setSelectedArtifactID] = useState<string | null>(null);
  const [pendingArtifactID, setPendingArtifactID] = useState<string | null>(null);
  const [failedArtifactID, setFailedArtifactID] = useState<string | null>(null);
  const [previewTrigger, setPreviewTrigger] = useState<HTMLButtonElement | null>(null);
  const requestRef = useRef<AbortController | null>(null);
  const hasLoadedEarlierRef = useRef(false);
  const items = useMemo(() => collectConversationImages([...earlierMessages, ...messages]), [earlierMessages, messages]);
  const selectedIndex = items.findIndex((item) => item.artifact.id === selectedArtifactID);

  useEffect(() => () => requestRef.current?.abort(), []);

  const openPreview = async (image: ConversationImage, trigger: HTMLButtonElement) => {
    if (requestRef.current !== null) return;
    setPreviewTrigger(trigger);
    setFailedArtifactID(null);
    if (hasMoreBefore && !hasLoadedEarlierRef.current) {
      const request = new AbortController();
      requestRef.current = request;
      setPendingArtifactID(image.artifact.id);
      try {
        const firstSeq = Math.min(...messages.map((message) => message.seq));
        const older = await loadEarlierConversationImages(conversationID, firstSeq, request.signal);
        if (request.signal.aborted) return;
        setEarlierMessages(older);
        hasLoadedEarlierRef.current = true;
      } catch {
        if (!request.signal.aborted) setFailedArtifactID(image.artifact.id);
        return;
      } finally {
        if (!request.signal.aborted) setPendingArtifactID(null);
        requestRef.current = null;
      }
    }
    setSelectedArtifactID(image.artifact.id);
  };

  return (
    <GalleryContext.Provider value={{ openPreview: (image, trigger) => void openPreview(image, trigger), pendingArtifactID, failedArtifactID }}>
      {children}
      {selectedIndex >= 0 ? (
        <FilePreviewDialog
          items={items}
          onClose={() => setSelectedArtifactID(null)}
          onSelect={(index) => {
            const item = items[index];
            if (item) setSelectedArtifactID(item.artifact.id);
          }}
          returnFocusTo={previewTrigger}
          selectedIndex={selectedIndex}
        />
      ) : null}
    </GalleryContext.Provider>
  );
}

export function ConversationAssistantMessage({ message }: Readonly<{ message: ConversationMessage }>) {
  const msg = useMessages();
  const t = useDictionary();
  const gallery = useContext(GalleryContext);
  const images = collectConversationImages([message]);

  return (
    <>
      <AssistantMessageContent markdown={message.text} omitImageArtifactIDs={images.map((image) => image.artifact.id)} />
      {message.videos?.map(video => <MediaVideo
        aria-label={msg("conversationImageGallery.generatedVideo")} className={styles.video} controls playsInline preload="metadata"
        key={video.id} src={`/web/v1/video-artifacts/${video.id}`}
      />)}
      {images.length > 0 && gallery ? (
        <ImageGenerationGrid count={images.length} aspectRatio={`${images[0].artifact.width || 1}:${images[0].artifact.height || 1}`}>
          {images.map((image) => (
            <div className={styles.image} key={image.artifact.id}>
              <FileCard
                isRetrying={false}
                job={image.job}
                onOpenPreview={(_job, _artifact, trigger) => gallery.openPreview(image, trigger)}
                onRequestResult={noAction}
                onRetryJob={noAction}
                result={{ job_id: image.job.id, status: "succeeded", artifacts: [image.artifact] }}
                resultState="idle"
                showDeleteControl={false}
              />
              {gallery.pendingArtifactID === image.artifact.id ? <StateNotice inline kind="loading">{t.conversations.imagesLoading}</StateNotice> : null}
              {gallery.failedArtifactID === image.artifact.id ? <StateNotice inline kind="error">{t.conversations.imagesLoadFailure}</StateNotice> : null}
            </div>
          ))}
        </ImageGenerationGrid>
      ) : null}
    </>
  );
}
