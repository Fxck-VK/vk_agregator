"use client";

import { MediaImage } from "@/components/media/MediaImage/MediaImage";
import { useEffect } from "react";
import { MediaPreviewDialogTemplate } from "@/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate";
import { useDictionary } from "@/i18n/LocaleProvider";

export type AttachmentPreviewItem = { id: string; src: string; alt: string };

type Props = {
  items: readonly AttachmentPreviewItem[];
  selectedIndex: number;
  onSelect: (index: number) => void;
  onClose: () => void;
  returnFocusTo?: HTMLElement | null;
};

// The owner retains the existing attachment URLs throughout the preview.
export function AttachmentPreviewDialog({ items, selectedIndex, onSelect, onClose, returnFocusTo }: Props) {
  const t = useDictionary();
  useEffect(() => () => {
    if (returnFocusTo?.isConnected) returnFocusTo.focus();
  }, [returnFocusTo]);

  return <MediaPreviewDialogTemplate
    ariaLabel={t.files.previewDialogLabel}
    backdropTestId="attachment-preview-backdrop"
    closeLabel={t.files.closePreview}
    getItemKey={item => item.id}
    getPreviewDimensions={() => ({ width: 1, height: 1 })}
    getThumbnailLabel={item => `${t.files.previewSelectFile}: ${item.alt}`}
    items={items}
    nextLabel={t.files.previewNextFile}
    onClose={onClose}
    onSelect={onSelect}
    previousLabel={t.files.previewPreviousFile}
    renderPreview={(item, classes) => <MediaImage key={item.id} src={item.src} alt={item.alt} fit="contain" className={classes.previewMedia} />}
    renderThumbnail={(item, classes) => <MediaImage src={item.src} alt="" fit="cover" passive className={classes.thumbnailMedia} />}
    selectedIndex={selectedIndex}
    testIdPrefix="attachment-preview"
    thumbnailRailLabel={t.files.previewFilesLabel}
  />;
}
