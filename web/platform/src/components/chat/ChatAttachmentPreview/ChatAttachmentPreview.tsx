"use client";

import { LoadingIndicator } from "@/components/ui/AsyncState/AsyncState";
import { RetryAction } from "@/components/ui/AsyncState/RetryAction";
import { MediaImage } from "@/components/media/MediaImage/MediaImage";
import { useId, useState, type CSSProperties } from "react";
import { FileIcon } from "@/components/icons/FileIcon";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import { Tooltip } from "@/components/ui/Tooltip/Tooltip";
import type { ChatAttachments } from "@/features/conversations/use-chat-attachments";
import { useDictionary, useMessages } from "@/i18n/LocaleProvider";
import styles from "./ChatAttachmentPreview.module.css";
import { measurePreviewCrop } from "./preview-crop";

type Props = {
  attachment: ChatAttachments["items"][number];
  disabled: boolean;
  onRemove: (id: string) => void;
  onRetry: (id: string) => void;
  onOpen?: (trigger: HTMLButtonElement) => void;
};

export function ChatAttachmentPreview({ attachment, disabled, onRemove, onRetry, onOpen }: Props) {
  const t = useDictionary();
  const msg = useMessages();
  const progress = attachment.progress;
  const errorId = useId();
  const [crop, setCrop] = useState<{ source: string | undefined; style: CSSProperties | undefined } | null>(null);
  return (
    <div className={styles.root} data-ui="chat-attachment" data-status={attachment.status}>
      <div className={styles.preview}>
        {attachment.previewUrl ? <MediaImage fit="cover" passive showLoading={attachment.status === "ready"}
          key={attachment.previewUrl}
          alt={attachment.name}
          src={attachment.previewUrl}
          style={crop?.source === attachment.previewUrl ? crop.style : undefined}
          onLoad={event => setCrop({ source: attachment.previewUrl, style: measurePreviewCrop(event.currentTarget) })}
        /> : (
          <div className={styles.file}><FileIcon /><span>{attachment.name}</span></div>
        )}
        {onOpen && attachment.previewUrl ? <button
          type="button"
          className={styles.openPreview}
          aria-label={`${t.files.previewDialogLabel}: ${attachment.name}`}
          aria-haspopup="dialog"
          onClick={event => onOpen(event.currentTarget)}
        /> : null}
        {attachment.status === "uploading" ? (
          <div className={styles.loading}>
            <LoadingIndicator
              label={msg("chatComposer.uploadingFile", { value1: attachment.name })}
              progress={progress}
              processingLabel={msg("chatComposer.processingFile")}
            />
          </div>
        ) : null}
      </div>
      {attachment.status === "failed" ? (
        <div className={styles.failure}>
          <RetryAction
            iconOnly label={msg("chatComposer.uploadFailedRetry")}
            aria-describedby={errorId}
            aria-label={msg("chatComposer.retryFileValue", { value1: attachment.name })}
            className={styles.retry} disabled={disabled} onClick={() => onRetry(attachment.id)}
          />
          <span className={styles.screenReaderError} id={errorId} role="alert">
            {attachment.error ?? msg("useChatAttachments.couldNotUploadTheFilePleaseTry")}
          </span>
        </div>
      ) : null}
      <div className={styles.remove}>
        <Tooltip label={msg("chatComposer.removeFile")}>
          <ModalCloseButton
            aria-label={msg("chatComposer.removeValue", { value1: attachment.name })}
            disabled={disabled}
            onClick={() => onRemove(attachment.id)}
            size="compact"
          />
        </Tooltip>
      </div>
    </div>
  );
}
