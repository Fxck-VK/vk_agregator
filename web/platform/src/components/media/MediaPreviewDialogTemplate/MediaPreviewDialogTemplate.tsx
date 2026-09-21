"use client";

import Image from "next/image";
import Link from "@/i18n/Link";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type CSSProperties,
  type ReactNode,
  type SyntheticEvent,
} from "react";

import { assetPaths } from "@/assets/asset-paths";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";

import styles from "./MediaPreviewDialogTemplate.module.css";

export type MediaPreviewDialogRenderClasses = {
  previewMedia: string;
  thumbnailMedia: string;
};

export type MediaPreviewPromptHeaderProps = {
  className?: string;
  copiedLabel: string;
  copyLabel: string;
  isCopied: boolean;
  onCopy: () => void;
  title: string;
};

export function MediaPreviewPromptHeader({
  className,
  copiedLabel,
  copyLabel,
  isCopied,
  onCopy,
  title,
}: Readonly<MediaPreviewPromptHeaderProps>) {
  return (
    <div className={[styles.promptHeader, className].filter(Boolean).join(" ")}>
      <h2>{title}</h2>
      <button
        aria-live="polite"
        className={styles.copyButton}
        onClick={onCopy}
        type="button"
      >
        {isCopied ? (
          <svg
            aria-hidden="true"
            className={[styles.copyIcon, styles.copySuccessIcon].join(" ")}
            data-testid="copy-success-icon"
            viewBox="0 0 24 24"
          >
            <path d="m5 12 4 4L19 6" />
          </svg>
        ) : (
          <Image
            alt=""
            aria-hidden="true"
            className={styles.copyIcon}
            height={24}
            src="/assets/icons/ui/copy-white.svg"
            unoptimized
            width={24}
          />
        )}
        {isCopied ? copiedLabel : copyLabel}
      </button>
    </div>
  );
}

export type MediaPreviewDialogActions = {
  download?: {
    ariaLabel?: string;
    download?: boolean | string;
    href: string;
    label: string;
  };
  feedback?: ReactNode;
  primary?: {
    ariaLabel?: string;
    href: string;
    label: string;
  };
  share?: {
    ariaLabel?: string;
    label: string;
    onClick: () => void;
  };
};

export type MediaPreviewDialogTemplateProps<T> = {
  ariaLabel: string;
  backdropTestId?: string;
  closeLabel: string;
  getActions?: (item: T) => MediaPreviewDialogActions | null;
  getItemKey: (item: T) => string;
  getPreviewDimensions: (item: T) => { height: number; width: number };
  getThumbnailLabel: (item: T) => string;
  infoPanel?: (item: T) => ReactNode;
  infoPanelTestId?: string;
  isItemSelectable?: (item: T) => boolean;
  items: readonly T[];
  nextLabel: string;
  onClose: () => void;
  onSelect: (index: number) => void;
  previousLabel: string;
  renderPreview: (item: T, classes: MediaPreviewDialogRenderClasses) => ReactNode;
  renderPreviewFooter?: (item: T) => ReactNode;
  renderThumbnail: (item: T, classes: MediaPreviewDialogRenderClasses) => ReactNode;
  selectedIndex: number;
  testIdPrefix: string;
  thumbnailRailLabel: string;
};

const narrowPreviewQuery = "(max-width: 59.999rem)";
const focusableSelector = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  '[tabindex]:not([tabindex="-1"])',
].join(",");

function subscribeToNarrowPreview(callback: () => void) {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return () => {};
  const mediaQuery = window.matchMedia(narrowPreviewQuery);
  mediaQuery.addEventListener("change", callback);
  return () => mediaQuery.removeEventListener("change", callback);
}

function getNarrowPreviewSnapshot() {
  return typeof window !== "undefined"
    && typeof window.matchMedia === "function"
    && window.matchMedia(narrowPreviewQuery).matches;
}

function isEditableKeyboardTarget(target: EventTarget | null) {
  return target instanceof HTMLInputElement
    || target instanceof HTMLTextAreaElement
    || target instanceof HTMLSelectElement
    || (target instanceof HTMLElement
      && (target.isContentEditable || target.closest('[role="textbox"]') !== null));
}

export function MediaPreviewDialogTemplate<T>({
  ariaLabel,
  backdropTestId,
  closeLabel,
  getActions,
  getItemKey,
  getPreviewDimensions,
  getThumbnailLabel,
  infoPanel,
  infoPanelTestId,
  isItemSelectable,
  items,
  nextLabel,
  onClose,
  onSelect,
  previousLabel,
  renderPreview,
  renderPreviewFooter,
  renderThumbnail,
  selectedIndex,
  testIdPrefix,
  thumbnailRailLabel,
}: Readonly<MediaPreviewDialogTemplateProps<T>>) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const selectedThumbnailRef = useRef<HTMLButtonElement>(null);
  const [loadedPreviewDimensions, setLoadedPreviewDimensions] = useState<{
    height: number;
    itemKey: string;
    width: number;
  } | null>(null);
  const isNarrowPreview = useSyncExternalStore(
    subscribeToNarrowPreview,
    getNarrowPreviewSnapshot,
    () => false,
  );
  const canSelectItem = useCallback(
    (item: T) => isItemSelectable?.(item) ?? true,
    [isItemSelectable],
  );
  const selectedIndexIsValid = Number.isInteger(selectedIndex)
    && selectedIndex >= 0
    && selectedIndex < items.length
    && canSelectItem(items[selectedIndex]!);
  const normalizedSelectedIndex = selectedIndexIsValid
    ? selectedIndex
    : items.findIndex(canSelectItem);
  const selectedItem = items[normalizedSelectedIndex];
  const selectableItemCount = items.filter(canSelectItem).length;

  useEffect(() => {
    dialogRef.current?.focus();
  }, []);

  useEffect(() => {
    selectedThumbnailRef.current?.scrollIntoView?.({ block: "nearest", inline: "nearest" });
  }, [normalizedSelectedIndex]);

  const selectRelativeItem = useCallback((offset: number) => {
    for (let step = 1; step <= items.length; step += 1) {
      const candidateIndex = (
        normalizedSelectedIndex + offset * step + items.length
      ) % items.length;
      if (canSelectItem(items[candidateIndex]!)) {
        onSelect(candidateIndex);
        return;
      }
    }
  }, [canSelectItem, items, normalizedSelectedIndex, onSelect]);

  useEffect(() => {
    const handleDocumentKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Tab") {
        const focusableElements = Array.from(
          dialogRef.current?.querySelectorAll<HTMLElement>(focusableSelector) ?? [],
        ).filter((element) => element.getAttribute("aria-hidden") !== "true");
        const firstElement = focusableElements[0];
        const lastElement = focusableElements.at(-1);
        if (firstElement === undefined || lastElement === undefined) return;

        if (event.shiftKey && document.activeElement === firstElement) {
          event.preventDefault();
          lastElement.focus();
        } else if (!event.shiftKey && document.activeElement === lastElement) {
          event.preventDefault();
          firstElement.focus();
        } else if (!dialogRef.current?.contains(document.activeElement)) {
          event.preventDefault();
          firstElement.focus();
        }
        return;
      }

      if (isEditableKeyboardTarget(event.target)) return;

      if (event.key === "ArrowLeft") {
        event.preventDefault();
        selectRelativeItem(-1);
      }

      if (event.key === "ArrowRight") {
        event.preventDefault();
        selectRelativeItem(1);
      }
    };

    document.addEventListener("keydown", handleDocumentKeyDown);
    return () => document.removeEventListener("keydown", handleDocumentKeyDown);
  }, [selectRelativeItem]);

  if (!selectedItem) return null;

  const selectedItemKey = getItemKey(selectedItem);
  const actions = getActions?.(selectedItem) ?? null;
  const hasActionButtons = Boolean(actions?.primary || actions?.download || actions?.share);
  const dimensions = loadedPreviewDimensions?.itemKey === selectedItemKey
    ? loadedPreviewDimensions
    : getPreviewDimensions(selectedItem);
  const aspectRatio = dimensions.width / dimensions.height;
  const renderClasses = {
    previewMedia: styles.previewMedia,
    thumbnailMedia: styles.thumbnailMedia,
  };
  const captureLoadedMediaDimensions = (event: SyntheticEvent<HTMLDivElement>) => {
    const dimensions = event.target instanceof HTMLImageElement
      ? { height: event.target.naturalHeight, width: event.target.naturalWidth }
      : event.target instanceof HTMLVideoElement
        ? { height: event.target.videoHeight, width: event.target.videoWidth }
        : null;
    if (!dimensions || dimensions.height <= 0 || dimensions.width <= 0) return;

    setLoadedPreviewDimensions({
      height: dimensions.height,
      itemKey: selectedItemKey,
      width: dimensions.width,
    });
  };

  return (
    <ModalBackdrop onClose={onClose} testId={backdropTestId}>
      {(requestClose) => (
        <div
          aria-label={ariaLabel}
          aria-modal="true"
          className={styles.dialog}
          data-single-item={items.length === 1}
          data-media-only={!infoPanel && !actions}
          ref={dialogRef}
          role="dialog"
          tabIndex={-1}
        >
          <ModalCloseButton
            aria-label={closeLabel}
            className={styles.closeButtonPlacement}
            onClick={requestClose}
          />

          {items.length > 1 ? <ScrollArea
            className={styles.thumbnailRail}
            orientation={isNarrowPreview ? "horizontal" : "vertical"}
            viewportAs="nav"
            viewportClassName={styles.thumbnailRailViewport}
            viewportProps={{ "aria-label": thumbnailRailLabel }}
          >
            {items.map((item, index) => {
              const isSelectable = canSelectItem(item);
              const isSelected = isSelectable && index === normalizedSelectedIndex;

              return (
                <button
                  aria-current={isSelected ? "true" : undefined}
                  aria-label={getThumbnailLabel(item)}
                  className={styles.thumbnail}
                  data-testid={`${testIdPrefix}-thumbnail`}
                  disabled={!isSelectable}
                  key={getItemKey(item)}
                  onClick={isSelectable ? () => onSelect(index) : undefined}
                  ref={isSelected ? selectedThumbnailRef : undefined}
                  type="button"
                >
                  {renderThumbnail(item, renderClasses)}
                </button>
              );
            })}
          </ScrollArea> : null}

          <div
            className={`${styles.preview} ${renderPreviewFooter ? styles.previewWithFooter : ""}`}
            data-testid={`${testIdPrefix}-preview`}
          >
            {selectableItemCount > 1 ? (
              <button
                aria-label={previousLabel}
                className={`${styles.previewNavigation} ${styles.previousButton}`}
                onClick={() => selectRelativeItem(-1)}
                type="button"
              >
                <Image
                  alt=""
                  aria-hidden="true"
                  className={styles.navigationIcon}
                  height={8}
                  src={assetPaths.icons.ui.faqArrow}
                  unoptimized
                  width={14}
                />
              </button>
            ) : null}
            <div className={styles.previewStage} data-testid={`${testIdPrefix}-media-stage`}>
              <div
                className={styles.previewSurface}
                data-landscape={aspectRatio > 1}
                data-testid={`${testIdPrefix}-media-surface`}
                onLoadCapture={captureLoadedMediaDimensions}
                onLoadedMetadataCapture={captureLoadedMediaDimensions}
                style={{
                  "--media-aspect": aspectRatio,
                  aspectRatio: `${dimensions.width} / ${dimensions.height}`,
                } as CSSProperties}
              >
                {renderPreview(selectedItem, renderClasses)}
              </div>
            </div>
            {selectableItemCount > 1 ? (
              <button
                aria-label={nextLabel}
                className={`${styles.previewNavigation} ${styles.nextButton}`}
                onClick={() => selectRelativeItem(1)}
                type="button"
              >
                <Image
                  alt=""
                  aria-hidden="true"
                  className={styles.navigationIcon}
                  height={8}
                  src={assetPaths.icons.ui.faqArrow}
                  unoptimized
                  width={14}
                />
              </button>
            ) : null}
            {renderPreviewFooter ? (
              <div className={styles.previewFooter}>
                {renderPreviewFooter(selectedItem)}
              </div>
            ) : null}
          </div>

          {infoPanel || actions ? <ScrollArea
            className={styles.infoPanel}
            viewportAs="aside"
            viewportClassName={styles.infoPanelViewport}
            viewportProps={infoPanelTestId ? { "data-testid": infoPanelTestId } : undefined}
          >
            {infoPanel?.(selectedItem)}
            {actions && (actions.feedback || hasActionButtons) ? (
              <div className={styles.previewActions}>
                {actions.feedback}
                {actions.primary ? (
                  <Link
                    aria-label={actions.primary.ariaLabel}
                    className={styles.primaryAction}
                    href={actions.primary.href}
                  >
                    {actions.primary.label}
                    <Image
                      alt=""
                      aria-hidden="true"
                      className={styles.previewActionIcon}
                      height={24}
                      src="/assets/icons/ui/star-white.svg"
                      unoptimized
                      width={24}
                    />
                  </Link>
                ) : null}
                {actions.download || actions.share ? (
                  <div className={styles.secondaryActions}>
                    {actions.download ? (
                      <a
                        aria-label={actions.download.ariaLabel}
                        className={styles.secondaryAction}
                        download={actions.download.download}
                        href={actions.download.href}
                      >
                        <Image
                          alt=""
                          aria-hidden="true"
                          className={styles.previewActionIcon}
                          height={24}
                          src="/assets/icons/ui/download-white.svg"
                          unoptimized
                          width={24}
                        />
                        {actions.download.label}
                      </a>
                    ) : null}
                    {actions.share ? (
                      <button
                        aria-label={actions.share.ariaLabel}
                        className={styles.secondaryAction}
                        onClick={actions.share.onClick}
                        type="button"
                      >
                        <Image
                          alt=""
                          aria-hidden="true"
                          className={styles.previewActionIcon}
                          height={24}
                          src="/assets/icons/ui/repost-white.svg"
                          unoptimized
                          width={24}
                        />
                        {actions.share.label}
                      </button>
                    ) : null}
                  </div>
                ) : null}
              </div>
            ) : null}
          </ScrollArea> : null}
        </div>
      )}
    </ModalBackdrop>
  );
}
