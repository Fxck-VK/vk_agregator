"use client";

import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useMessages, useDictionary } from "@/i18n/LocaleProvider";

import {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from "react";

import {
  MediaPreviewDialogTemplate,
  MediaPreviewPromptHeader,
  type MediaPreviewDialogRenderClasses,
} from "@/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate";
import { ModelIcon } from "@/features/models/ModelIcon/ModelIcon";

import { InspirationExampleMedia } from "../InspirationExampleMedia/InspirationExampleMedia";
import type { InspirationExample } from "../inspiration-examples";
import styles from "./InspirationExampleCard.module.css";

type Feedback = "copyFailure" | "linkCopied" | "shareFailure" | null;

type InspirationExampleDialogProps = {
  examples: readonly InspirationExample[];
  onClose: () => void;
  onSelect: (index: number) => void;
  selectedIndex: number;
};

export function InspirationExampleDialog({
  examples,
  onClose,
  onSelect,
  selectedIndex,
}: Readonly<InspirationExampleDialogProps>) {
  const msg = useMessages();
  const t = useDictionary();
  const [feedback, setFeedback] = useState<Feedback>(null);
  const [isPromptCopied, setIsPromptCopied] = useState(false);
  const [isPromptExpanded, setIsPromptExpanded] = useState(false);
  const [isPromptOverflowing, setIsPromptOverflowing] = useState(false);
  const promptRef = useRef<HTMLParagraphElement>(null);
  const previewVideoRef = useRef<HTMLVideoElement>(null);
  const promptId = useId();
  const example = examples[selectedIndex] ?? examples[0]!;
  const recreateHref = useMemo(() => {
    if (!example.canRecreate) return null;

    const params = new URLSearchParams({
      model: example.modelId,
      prompt: example.prompt,
      quality: example.quality,
    });
    return `/app/image?${params.toString()}`;
  }, [example.canRecreate, example.modelId, example.prompt, example.quality]);

  useEffect(() => {
    // Preserve the legacy dialog's immediate reset when the selected example changes.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setIsPromptCopied(false);
    setIsPromptExpanded(false);
    setIsPromptOverflowing(false);
  }, [example.id]);

  useEffect(() => {
    if (!isPromptCopied) return;

    const timeoutId = window.setTimeout(() => setIsPromptCopied(false), 2_000);
    return () => window.clearTimeout(timeoutId);
  }, [isPromptCopied]);

  useEffect(() => {
    if (isPromptExpanded) return;

    const measurePrompt = () => {
      const prompt = promptRef.current;
      if (!prompt) return;
      setIsPromptOverflowing(prompt.scrollHeight > prompt.clientHeight + 1);
    };

    measurePrompt();
    window.addEventListener("resize", measurePrompt);
    return () => window.removeEventListener("resize", measurePrompt);
  }, [example.id, isPromptExpanded]);

  useEffect(() => {
    const video = previewVideoRef.current;
    if (!video) return;

    video.currentTime = 0;
    video.muted = true;
    void video.play().catch(() => undefined);

    return () => {
      video.pause();
      video.currentTime = 0;
    };
  }, [example.id]);

  const selectExample = (index: number) => {
    setFeedback(null);
    setIsPromptCopied(false);
    onSelect(index);
  };

  const copyPrompt = async () => {
    try {
      await navigator.clipboard.writeText(example.prompt);
      setFeedback(null);
      setIsPromptCopied(true);
    } catch {
      setIsPromptCopied(false);
      setFeedback("copyFailure");
    }
  };

  const shareExample = async () => {
    const shareData = {
      title: example.title,
      text: example.prompt,
      url: window.location.href,
    };

    try {
      if (navigator.share) {
        await navigator.share(shareData);
        return;
      }
      await navigator.clipboard.writeText(shareData.url);
      setFeedback("linkCopied");
    } catch {
      setFeedback("shareFailure");
    }
  };

  const feedbackText = feedback
    ? {
        copyFailure: t.inspiration.copyFailure,
        linkCopied: t.inspiration.linkCopied,
        shareFailure: t.inspiration.shareFailure,
      }[feedback]
    : null;

  const renderThumbnail = (
    item: InspirationExample,
    classes: MediaPreviewDialogRenderClasses,
  ) => (
    <InspirationExampleMedia
      className={classes.thumbnailMedia}
      example={item}
      videoProps={{ muted: true, playsInline: true, preload: "none" }}
    />
  );

  const renderPreview = (
    item: InspirationExample,
    classes: MediaPreviewDialogRenderClasses,
  ) => (
    <InspirationExampleMedia
      className={classes.previewMedia} fit="contain" passive={false}
      example={item}
      key={item.id}
      priority
      videoProps={{ controls: true, muted: true, playsInline: true, preload: "metadata" }}
      videoRef={previewVideoRef}
    />
  );

  const renderInfoPanel = (item: InspirationExample) => (
    <>
      <div className={styles.modelTitle}>
        <ModelIcon className={styles.modelMark} />
        <div>
          <strong>{item.modelName}</strong>
        </div>
      </div>

      <MediaPreviewPromptHeader
        className={styles.promptHeaderSlot}
        copiedLabel={t.inspiration.copied}
        copyLabel={t.inspiration.copyPrompt}
        isCopied={isPromptCopied}
        onCopy={() => void copyPrompt()}
        title={t.inspiration.promptTitle}
      />
      <div className={styles.promptBlock}>
        <p
          className={`${styles.prompt} ${isPromptExpanded ? "" : styles.promptCollapsed}`}
          id={promptId}
          ref={promptRef}
        >
          {item.prompt}
          {isPromptOverflowing ? (
            <>
              {" "}
              <button
                aria-controls={promptId}
                aria-expanded={isPromptExpanded}
                className={[
                  styles.promptToggle,
                  isPromptExpanded
                    ? styles.promptToggleExpanded
                    : styles.promptToggleCollapsed,
                ].join(" ")}
                onClick={() => setIsPromptExpanded((expanded) => !expanded)}
                type="button"
              >
                {isPromptExpanded ? t.inspiration.promptCollapse : t.inspiration.promptShowMore}
              </button>
            </>
          ) : null}
        </p>
      </div>
    </>
  );

  return (
    <MediaPreviewDialogTemplate
      ariaLabel={t.inspiration.dialogLabel}
      closeLabel={t.inspiration.close}
      getActions={(item) => ({
        download: {
          download: item.downloadName,
          href: item.mediaPath,
          label: t.inspiration.download,
        },
        feedback: feedbackText
          ? <StateNotice inline kind="success">{feedbackText}</StateNotice>
          : null,
        primary: recreateHref
          ? {
              href: recreateHref,
              label: t.inspiration.recreate,
            }
          : undefined,
        share: {
          label: t.inspiration.share,
          onClick: () => void shareExample(),
        },
      })}
      getItemKey={(item) => item.id}
      getPreviewDimensions={(item) => ({
        height: item.mediaHeight,
        width: item.mediaWidth,
      })}
      getThumbnailLabel={(item) => msg("inspirationExampleDialogTemplate.showTheValueExample", { value1: item.title })}
      infoPanel={renderInfoPanel}
      items={examples}
      nextLabel={t.inspiration.nextExample}
      onClose={onClose}
      onSelect={selectExample}
      previousLabel={t.inspiration.previousExample}
      renderPreview={renderPreview}
      renderThumbnail={renderThumbnail}
      selectedIndex={selectedIndex}
      testIdPrefix="inspiration"
      thumbnailRailLabel={t.inspiration.examplesLabel}
    />
  );
}
