"use client";

import { MediaState, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { MediaImage } from "@/components/media/MediaImage/MediaImage";
import { useMessages, useDictionary } from "@/i18n/LocaleProvider";

import { getTranslator, type Translator } from "@/i18n/messages";

import { useEffect, useRef, useState } from "react";
import Image from "next/image";

import {
  MediaPreviewDialogTemplate,
  MediaPreviewPromptHeader,
  type MediaPreviewDialogRenderClasses,
} from "@/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate";
import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { getModelPresentation } from "@/features/models/ModelCard/model-card-content";
import { ModelIcon } from "@/features/models/ModelIcon/ModelIcon";

import type { ImageJob, ImageJobResult } from "@/lib/web-api/contracts";

import {
  FileAnimationPanel,
  FileBackgroundRemovalPanel,
  FileEnhancementPanel,
} from "./FileAnimationPanel";
import {
  FileEditPreview,
  FileEditorPanel,
  useFileEditorController,
} from "./FileEditorPanel";
import styles from "./FilePreviewDialog.module.css";

type FilePreviewArtifact = ImageJobResult["artifacts"][number];
type ReadyFilePreviewItem = { artifact: FilePreviewArtifact; job: ImageJob; state?: "ready" };

export type FilePreviewItem =
  | ReadyFilePreviewItem
  | { job: ImageJob; state: "loading" | "unavailable" };

type FilePreviewDialogProps = {
  items: readonly FilePreviewItem[];
  onClose: () => void;
  onSelect: (index: number) => void;
  returnFocusTo?: HTMLElement | null;
  selectedIndex: number;
};

type ReadyFilePreviewDialogProps = FilePreviewDialogProps & {
  selectedItem: ReadyFilePreviewItem;
};

type ShareFeedback = "copied" | "copyFailure" | "failure" | null;

type PreviewToolID = "general" | "animate" | "enhance" | "remove-background" | "edit";

const toolIconSources: Record<PreviewToolID, string> = {
  animate: "/assets/icons/ui/animate-white.svg",
  edit: "/assets/icons/ui/edit-white.svg",
  enhance: "/assets/icons/ui/enhance-white.svg",
  general: "/assets/icons/ui/general-white.svg",
  "remove-background": "/assets/icons/ui/remove-background-white.svg",
};

function isReadyPreviewItem(item: FilePreviewItem | undefined): item is ReadyFilePreviewItem {
  return item !== undefined && "artifact" in item;
}

function getReadyPreviewItem(item: FilePreviewItem): ReadyFilePreviewItem {
  if (!isReadyPreviewItem(item)) {
    throw new Error("Only ready files can be rendered in the preview stage.");
  }
  return item;
}

function formatCreatedAt(value: string, msg: Translator) {
  return new Intl.DateTimeFormat(msg.locale, {
    dateStyle: "medium",
    timeZone: "UTC",
  }).format(new Date(value));
}

function formatMimeType(value: string) {
  return value.split("/").at(-1)?.toUpperCase() ?? value.toUpperCase();
}

function formatFileSize(bytes: number, msg: Translator = getTranslator("ru")) {
  if (bytes < 1024) return msg("filePreviewDialog.valueB", { value1: bytes });

  const units = [msg("filePreviewDialog.kb"), msg("filePreviewDialog.mb"), msg("filePreviewDialog.gb")] as const;
  let value = bytes / 1024;
  let unitIndex = 0;

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }

  return `${new Intl.NumberFormat(msg.locale, { maximumFractionDigits: 1 }).format(value)} ${units[unitIndex]}`;
}

function getRecreateHref(item: ReadyFilePreviewItem) {
  const params = new URLSearchParams({
    model: item.job.model_id,
    prompt: item.job.prompt,
    quality: item.job.image_quality,
  });
  return `/app/image?${params.toString()}`;
}

export function FilePreviewDialog(props: Readonly<FilePreviewDialogProps>) {
  const requestedItem = props.items[props.selectedIndex] ?? props.items[0];
  const selectedItem = isReadyPreviewItem(requestedItem)
    ? requestedItem
    : props.items.find(isReadyPreviewItem);

  if (!selectedItem) return null;

  return <ReadyFilePreviewDialog {...props} selectedItem={selectedItem} />;
}

function ReadyFilePreviewDialog({
  items,
  onClose,
  onSelect,
  returnFocusTo,
  selectedItem,
  selectedIndex,
}: Readonly<ReadyFilePreviewDialogProps>) {
  const msg = useMessages();
  const t = useDictionary();
  const toolActions = [
    { id: "general", label: t.files.previewGeneral },
    { id: "animate", label: msg("filePreviewDialog.animate") },
    { id: "enhance", label: msg("filePreviewDialog.enhance") },
    { id: "remove-background", label: msg("filePreviewDialog.removeBackground") },
    { id: "edit", label: msg("filePreviewDialog.edit") },
  ] as const;
  const toolItems = toolActions.map((action) => ({
    ...action,
    icon: (
      <Image
        alt=""
        aria-hidden="true"
        height={20}
        src={toolIconSources[action.id]}
        unoptimized
        width={20}
      />
    ),
  }));

  const [shareFeedback, setShareFeedback] = useState<ShareFeedback>(null);
  const [isPromptCopied, setIsPromptCopied] = useState(false);
  const [isPromptExpanded, setIsPromptExpanded] = useState(false);
  const [isPromptOverflowing, setIsPromptOverflowing] = useState(false);
  const [activeToolID, setActiveToolID] = useState<PreviewToolID>("general");
  const editorController = useFileEditorController();
  const promptRef = useRef<HTMLParagraphElement>(null);
  const { artifact, job } = selectedItem;
  const feedbackText = shareFeedback === "copied"
    ? t.files.previewLinkCopied
    : shareFeedback === "copyFailure"
      ? t.files.previewCopyFailure
      : shareFeedback === "failure"
        ? t.files.previewShareFailure
        : null;

  const selectItem = (index: number) => {
    setIsPromptCopied(false);
    setIsPromptExpanded(false);
    setShareFeedback(null);
    editorController.reset();
    onSelect(index);
  };

  useEffect(() => {
    if (isPromptExpanded) return;
    const measurePrompt = () => {
      const prompt = promptRef.current;
      setIsPromptOverflowing(Boolean(prompt && prompt.scrollHeight > prompt.clientHeight + 1));
    };
    measurePrompt();
    window.addEventListener("resize", measurePrompt);
    return () => window.removeEventListener("resize", measurePrompt);
  }, [artifact.id, isPromptExpanded, job.prompt]);

  useEffect(() => () => {
    if (returnFocusTo?.isConnected) returnFocusTo.focus();
  }, [returnFocusTo]);

  useEffect(() => {
    if (!isPromptCopied) return;
    const timeoutID = window.setTimeout(() => setIsPromptCopied(false), 2_000);
    return () => window.clearTimeout(timeoutID);
  }, [isPromptCopied]);

  const shareFile = async (item: ReadyFilePreviewItem) => {
    const artifactPath = `/web/v1/image-artifacts/${item.artifact.id}`;
    const artifactURL = new URL(artifactPath, window.location.origin).toString();

    try {
      if (navigator.share) {
        await navigator.share({ title: item.job.prompt, url: artifactURL });
        return;
      }
      await navigator.clipboard.writeText(artifactURL);
      setShareFeedback("copied");
    } catch {
      setShareFeedback("failure");
    }
  };

  const copyPrompt = async (item: ReadyFilePreviewItem) => {
    try {
      if (!navigator.clipboard) throw new Error("Clipboard API is unavailable");
      await navigator.clipboard.writeText(item.job.prompt);
      setShareFeedback(null);
      setIsPromptCopied(true);
    } catch {
      setIsPromptCopied(false);
      setShareFeedback("copyFailure");
    }
  };

  const getThumbnailLabel = (item: FilePreviewItem) => {
    const siblingItems = items.filter((candidate) => candidate.job.id === item.job.id);
    const itemPosition = siblingItems.indexOf(item) + 1;
    const positionLabel = siblingItems.length > 1
      ? msg("filePreviewDialog.valueOfValue", { value1: itemPosition, value2: siblingItems.length })
      : "";
    const prefix = isReadyPreviewItem(item)
      ? t.files.previewSelectFile
      : item.state === "loading"
        ? t.files.previewLoadingFile
        : t.files.previewUnavailableFile;
    return `${prefix}: ${item.job.prompt}${positionLabel}`;
  };

  const renderThumbnail = (
    item: FilePreviewItem,
    classes: MediaPreviewDialogRenderClasses,
  ) => isReadyPreviewItem(item) ? (
    <MediaImage
      alt=""
      aria-hidden="true"
      fit="cover" passive className={classes.thumbnailMedia}
      src={`/web/v1/image-artifacts/${item.artifact.id}?preview=1`}
      loading="lazy"
    />
  ) : (
    <MediaState compact state={item.state === "loading" ? "loading" : "error"} label={item.state === "loading" ? t.files.previewLoading : t.files.previewFailure} />
  );

  const renderPreview = (
    item: FilePreviewItem,
    classes: MediaPreviewDialogRenderClasses,
  ) => {
    const readyItem = getReadyPreviewItem(item);
    const imageSource = `/web/v1/image-artifacts/${readyItem.artifact.id}`;
    if (activeToolID === "edit") {
      return (
        <FileEditPreview
          alt={readyItem.job.prompt}
          controller={editorController}
          imageClassName={classes.previewMedia}
          src={imageSource}
        />
      );
    }
    return (
      <MediaImage
        alt={readyItem.job.prompt}
        fit="contain" className={classes.previewMedia}
        src={imageSource}
      />
    );
  };

  const renderToolRail = () => (
    <ModeSwitchPanel
      activeID={activeToolID}
      ariaLabel={t.files.previewToolsLabel}
      className={styles.toolRail}
      items={toolItems}
      onChange={setActiveToolID}
    />
  );

  const renderInfoPanel = (item: FilePreviewItem) => {
    const readyItem = getReadyPreviewItem(item);
    const presentation = getModelPresentation({
      id: readyItem.job.model_id,
      name: readyItem.job.model_name,
    }, msg);

    return (
      <>
        <header className={styles.modelHeader}>
          <ModelIcon className={styles.modelIcon} src={presentation.artworkSrc} />
          <strong>{readyItem.job.model_name}</strong>
        </header>

        <section>
          <MediaPreviewPromptHeader
            copiedLabel={t.files.previewPromptCopied}
            copyLabel={t.files.previewCopyPrompt}
            isCopied={isPromptCopied}
            onCopy={() => void copyPrompt(readyItem)}
            title={t.files.previewPromptTitle}
          />
          <div className={styles.promptBody}>
            <p
              className={isPromptExpanded ? styles.promptExpanded : styles.promptCollapsed}
              ref={promptRef}
            >
              {readyItem.job.prompt}
            </p>
            {isPromptOverflowing || isPromptExpanded ? (
              <button
                className={`${styles.promptToggle} ${
                  isPromptExpanded ? styles.promptToggleExpanded : styles.promptToggleCollapsed
                }`}
                onClick={() => setIsPromptExpanded((current) => !current)}
                type="button"
              >
                {isPromptExpanded ? t.files.previewPromptCollapse : t.files.previewPromptShowMore}
              </button>
            ) : null}
          </div>
        </section>

        <dl className={styles.metadata}>
          <div>
            <dt>{t.files.previewCreatedAt}</dt>
            <dd>{formatCreatedAt(readyItem.job.created_at, msg)}</dd>
          </div>
          {readyItem.artifact.width > 0 && readyItem.artifact.height > 0 ? (
            <div>
              <dt>{t.files.previewResolution}</dt>
              <dd>{readyItem.artifact.width} × {readyItem.artifact.height}</dd>
            </div>
          ) : null}
          <div>
            <dt>{t.files.previewFormat}</dt>
            <dd>{formatMimeType(readyItem.artifact.mime_type)}</dd>
          </div>
          <div>
            <dt>{t.files.previewSize}</dt>
            <dd>{formatFileSize(readyItem.artifact.size_bytes, msg)}</dd>
          </div>
          <div>
            <dt>{t.files.previewQuality}</dt>
            <dd>{readyItem.job.image_quality}</dd>
          </div>
        </dl>

      </>
    );
  };

  return (
    <MediaPreviewDialogTemplate
      ariaLabel={`${t.files.previewDialogLabel}: ${job.prompt}`}
      backdropTestId="file-preview-backdrop"
      closeLabel={t.files.closePreview}
      getActions={(item) => {
        if (
          activeToolID === "animate"
          || activeToolID === "enhance"
          || activeToolID === "remove-background"
          || activeToolID === "edit"
        ) return null;
        const readyItem = getReadyPreviewItem(item);
        return {
          download: {
            ariaLabel: t.files.previewDownloadLabel,
            download: true,
            href: `/web/v1/image-artifacts/${readyItem.artifact.id}`,
            label: t.files.download,
          },
          feedback: feedbackText
            ? <StateNotice inline kind="success">{feedbackText}</StateNotice>
            : null,
          primary: {
            href: getRecreateHref(readyItem),
            label: t.files.previewRecreate,
          },
          share: {
            ariaLabel: t.files.previewShareLabel,
            label: t.files.previewShare,
            onClick: () => void shareFile(readyItem),
          },
        };
      }}
      getItemKey={(item) => isReadyPreviewItem(item)
        ? item.artifact.id
        : `${item.job.id}-${item.state}`}
      getPreviewDimensions={(item) => {
        const readyItem = getReadyPreviewItem(item);
        return {
          height: readyItem.artifact.height || 1,
          width: readyItem.artifact.width || 1,
        };
      }}
      getThumbnailLabel={getThumbnailLabel}
      infoPanel={activeToolID === "animate"
        ? () => <FileAnimationPanel />
        : activeToolID === "enhance"
          ? () => <FileEnhancementPanel />
          : activeToolID === "remove-background"
            ? () => <FileBackgroundRemovalPanel />
            : activeToolID === "edit"
              ? () => <FileEditorPanel controller={editorController} />
          : renderInfoPanel}
      infoPanelTestId="file-preview-info-viewport"
      isItemSelectable={isReadyPreviewItem}
      items={items}
      nextLabel={t.files.previewNextFile}
      onClose={onClose}
      onSelect={selectItem}
      previousLabel={t.files.previewPreviousFile}
      renderPreview={renderPreview}
      renderPreviewFooter={renderToolRail}
      renderThumbnail={renderThumbnail}
      selectedIndex={selectedIndex}
      testIdPrefix="file-preview"
      thumbnailRailLabel={t.files.previewFilesLabel}
    />
  );
}
