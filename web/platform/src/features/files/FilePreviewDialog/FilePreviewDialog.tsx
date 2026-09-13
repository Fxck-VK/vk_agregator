"use client";

/* eslint-disable @next/next/no-img-element */

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
import { ru } from "@/i18n/ru";
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

const toolActions = [
  { id: "general", label: ru.files.previewGeneral },
  { id: "animate", label: "Оживить" },
  { id: "enhance", label: "Улучшить" },
  { id: "remove-background", label: "Удалить фон" },
  { id: "edit", label: "Редактировать" },
] as const;

type PreviewToolID = (typeof toolActions)[number]["id"];

const toolIconSources: Record<PreviewToolID, string> = {
  animate: "/assets/icons/ui/animate-white.svg",
  edit: "/assets/icons/ui/edit-white.svg",
  enhance: "/assets/icons/ui/enhance-white.svg",
  general: "/assets/icons/ui/general-white.svg",
  "remove-background": "/assets/icons/ui/remove-background-white.svg",
};

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

function isReadyPreviewItem(item: FilePreviewItem | undefined): item is ReadyFilePreviewItem {
  return item !== undefined && "artifact" in item;
}

function getReadyPreviewItem(item: FilePreviewItem): ReadyFilePreviewItem {
  if (!isReadyPreviewItem(item)) {
    throw new Error("Only ready files can be rendered in the preview stage.");
  }
  return item;
}

function formatCreatedAt(value: string) {
  return new Intl.DateTimeFormat("ru-RU", {
    dateStyle: "medium",
    timeZone: "UTC",
  }).format(new Date(value));
}

function formatMimeType(value: string) {
  return value.split("/").at(-1)?.toUpperCase() ?? value.toUpperCase();
}

function formatFileSize(bytes: number) {
  if (bytes < 1024) return `${bytes} Б`;

  const units = ["КБ", "МБ", "ГБ"] as const;
  let value = bytes / 1024;
  let unitIndex = 0;

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }

  return `${new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 1 }).format(value)} ${units[unitIndex]}`;
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
  const [shareFeedback, setShareFeedback] = useState<ShareFeedback>(null);
  const [isPromptCopied, setIsPromptCopied] = useState(false);
  const [isPromptExpanded, setIsPromptExpanded] = useState(false);
  const [isPromptOverflowing, setIsPromptOverflowing] = useState(false);
  const [activeToolID, setActiveToolID] = useState<PreviewToolID>("general");
  const editorController = useFileEditorController();
  const promptRef = useRef<HTMLParagraphElement>(null);
  const { artifact, job } = selectedItem;
  const feedbackText = shareFeedback === "copied"
    ? ru.files.previewLinkCopied
    : shareFeedback === "copyFailure"
      ? ru.files.previewCopyFailure
      : shareFeedback === "failure"
        ? ru.files.previewShareFailure
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
      ? ` (${itemPosition} из ${siblingItems.length})`
      : "";
    const prefix = isReadyPreviewItem(item)
      ? ru.files.previewSelectFile
      : item.state === "loading"
        ? ru.files.previewLoadingFile
        : ru.files.previewUnavailableFile;
    return `${prefix}: ${item.job.prompt}${positionLabel}`;
  };

  const renderThumbnail = (
    item: FilePreviewItem,
    classes: MediaPreviewDialogRenderClasses,
  ) => isReadyPreviewItem(item) ? (
    <img
      alt=""
      aria-hidden="true"
      className={classes.thumbnailMedia}
      src={`/web/v1/image-artifacts/${item.artifact.id}`}
    />
  ) : (
    <span aria-hidden="true" className={styles.thumbnailPlaceholderMark}>
      {item.state === "loading" ? "…" : "!"}
    </span>
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
      <img
        alt={readyItem.job.prompt}
        className={classes.previewMedia}
        src={imageSource}
      />
    );
  };

  const renderToolRail = () => (
    <ModeSwitchPanel
      activeID={activeToolID}
      ariaLabel={ru.files.previewToolsLabel}
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
    });

    return (
      <>
        <header className={styles.modelHeader}>
          <ModelIcon className={styles.modelIcon} src={presentation.artworkSrc} />
          <strong>{readyItem.job.model_name}</strong>
        </header>

        <section>
          <MediaPreviewPromptHeader
            copiedLabel={ru.files.previewPromptCopied}
            copyLabel={ru.files.previewCopyPrompt}
            isCopied={isPromptCopied}
            onCopy={() => void copyPrompt(readyItem)}
            title={ru.files.previewPromptTitle}
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
                {isPromptExpanded ? ru.files.previewPromptCollapse : ru.files.previewPromptShowMore}
              </button>
            ) : null}
          </div>
        </section>

        <dl className={styles.metadata}>
          <div>
            <dt>{ru.files.previewCreatedAt}</dt>
            <dd>{formatCreatedAt(readyItem.job.created_at)}</dd>
          </div>
          {readyItem.artifact.width > 0 && readyItem.artifact.height > 0 ? (
            <div>
              <dt>{ru.files.previewResolution}</dt>
              <dd>{readyItem.artifact.width} × {readyItem.artifact.height}</dd>
            </div>
          ) : null}
          <div>
            <dt>{ru.files.previewFormat}</dt>
            <dd>{formatMimeType(readyItem.artifact.mime_type)}</dd>
          </div>
          <div>
            <dt>{ru.files.previewSize}</dt>
            <dd>{formatFileSize(readyItem.artifact.size_bytes)}</dd>
          </div>
          <div>
            <dt>{ru.files.previewQuality}</dt>
            <dd>{readyItem.job.image_quality}</dd>
          </div>
        </dl>

      </>
    );
  };

  return (
    <MediaPreviewDialogTemplate
      ariaLabel={`${ru.files.previewDialogLabel}: ${job.prompt}`}
      backdropTestId="file-preview-backdrop"
      closeLabel={ru.files.closePreview}
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
            ariaLabel: ru.files.previewDownloadLabel,
            download: true,
            href: `/web/v1/image-artifacts/${readyItem.artifact.id}`,
            label: ru.files.download,
          },
          feedback: feedbackText
            ? <p className={styles.feedback} role="status">{feedbackText}</p>
            : null,
          primary: {
            href: getRecreateHref(readyItem),
            label: ru.files.previewRecreate,
          },
          share: {
            ariaLabel: ru.files.previewShareLabel,
            label: ru.files.previewShare,
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
      nextLabel={ru.files.previewNextFile}
      onClose={onClose}
      onSelect={selectItem}
      previousLabel={ru.files.previewPreviousFile}
      renderPreview={renderPreview}
      renderPreviewFooter={renderToolRail}
      renderThumbnail={renderThumbnail}
      selectedIndex={selectedIndex}
      testIdPrefix="file-preview"
      thumbnailRailLabel={ru.files.previewFilesLabel}
    />
  );
}
