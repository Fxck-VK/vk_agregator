"use client";

import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useMessages } from "@/i18n/LocaleProvider";


import { useCallback, useRef, useState, type ChangeEventHandler, type ReactNode } from "react";

import {
  attachmentFromFile,
  ChatFilePicker,
  type ChatFileSource,
} from "@/components/chat/ChatFilePicker/ChatFilePicker";
import { ChatMediaMenu, type ChatMediaMenuLabels } from "@/components/chat/ChatMediaMenu/ChatMediaMenu";
import { ChatSubmitButton } from "@/components/chat/ChatSubmitButton/ChatSubmitButton";
import { ChatTextInput } from "@/components/chat/ChatTextInput/ChatTextInput";
import { useWorkspaceFileDrop } from "@/components/chat/WorkspaceFileDropZone/WorkspaceFileDropZone";
import { useChatAttachments, type ChatAttachments } from "@/features/conversations/use-chat-attachments";
import { ChatAttachmentPreview } from "@/components/chat/ChatAttachmentPreview/ChatAttachmentPreview";
import { ChatAttachmentNotice } from "@/components/chat/ChatAttachmentNotice/ChatAttachmentNotice";
import { ChatUploadNetworkNotice } from "@/components/chat/ChatUploadNetworkNotice/ChatUploadNetworkNotice";
import { AttachmentPreviewDialog } from "@/components/media/AttachmentPreviewDialog/AttachmentPreviewDialog";
import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import styles from "./ChatComposer.module.css";

export type ChatComposerVariant = "conversation" | "hero" | "newChat" | "workspace";

type ChatComposerProps = {
  attachmentsEnabled?: boolean;
  attachmentController?: ChatAttachments;
  additionalControls?: ReactNode;
  canSubmit: boolean;
  disabled: boolean;
  generatedMediaHref?: string;
  label: string;
  leadingControls?: ReactNode;
  mediaLabel: string;
  mediaLibraryEnabled?: boolean;
  mediaMenuLabels?: Omit<ChatMediaMenuLabels, "trigger">;
  note?: ReactNode;
  onChooseGeneratedMedia?: () => void;
  onChooseUploadedMedia?: () => void;
  onChange: ChangeEventHandler<HTMLTextAreaElement>;
  onFilesSelected?: (files: File[]) => void;
  onSend: () => void;
  placeholder: string;
  submitLabel: string;
  uploadedMediaHref?: string;
  value: string;
  variant: ChatComposerVariant;
  wrapLeadingControls?: boolean;
};

const expandedVariants = new Set<ChatComposerVariant>(["hero", "workspace"]);

export function ChatComposer({
  attachmentsEnabled = true,
  attachmentController,
  additionalControls,
  canSubmit,
  disabled,
  generatedMediaHref,
  label,
  leadingControls,
  mediaLabel,
  mediaLibraryEnabled = true,
  mediaMenuLabels,
  note,
  onChooseGeneratedMedia,
  onChooseUploadedMedia,
  onChange,
  onFilesSelected,
  onSend,
  placeholder,
  submitLabel,
  uploadedMediaHref,
  value,
  variant,
  wrapLeadingControls = false,
}: ChatComposerProps) {
  const msg = useMessages();
  const isLandingComposer = variant === "hero" || variant === "newChat";
  const isExpanded = expandedVariants.has(variant);
  const fallbackAttachments = useChatAttachments();
  const attachmentState = attachmentController ?? fallbackAttachments;
  const { items: attachments, add, remove, retry, error, blocked } = attachmentState;
  const [pickerSource, setPickerSource] = useState<Exclude<ChatFileSource, "all"> | null>(null);
  const inputSurfaceRef = useRef<HTMLDivElement>(null);
  const [preview, setPreview] = useState<{ id: string; trigger: HTMLButtonElement } | null>(null);
  const previewItems = attachments.flatMap(item => item.previewUrl
    ? [{ id: item.id, src: item.previewUrl, alt: item.name }] : []);
  const previewIndex = previewItems.findIndex(item => item.id === preview?.id);
  const selectNativeFile = useCallback((files: File[]) => {
    if (disabled) return;
    add(files.map(attachmentFromFile));
    onFilesSelected?.(files);
  }, [add, disabled, onFilesSelected]);
  useWorkspaceFileDrop(selectNativeFile, !disabled);
  const send = () => {
    if (canSubmit && !disabled && !blocked) onSend();
  };
  const openUploadedPicker = () => {
    setPickerSource("uploaded");
    onChooseUploadedMedia?.();
  };
  const openGeneratedPicker = () => {
    setPickerSource("generated");
    onChooseGeneratedMedia?.();
  };

  return (
    <>
      <InputSurface
        ref={inputSurfaceRef}
        className={`${styles.surface} ${styles[variant]}`}
        data-wrap-leading-controls={wrapLeadingControls || undefined}
        data-has-attachments={attachments.length > 0 || !!error || undefined}
        onPaste={event => {
          const files = Array.from(event.clipboardData.files);
          if (!files.length) return;
          event.preventDefault();
          selectNativeFile(files);
        }}
      >
        {attachments.length === 0 && !error ? null : (
          <div className={styles.attachments}>
            <div className={styles.attachmentList}>
              {attachments.map((attachment) => (
                <ChatAttachmentPreview key={attachment.id} attachment={attachment} disabled={disabled} onRemove={remove} onRetry={retry}
                  onOpen={trigger => setPreview({ id: attachment.id, trigger })} />
              ))}
            </div>
            {error ? <StateNotice inline kind="error">{error}</StateNotice> : null}
          </div>
        )}
        <label className={styles.field}>
          <span>{label}</span>
          <ChatTextInput
            appearance="composer"
            disabled={disabled}
            onChange={onChange}
            onSend={send}
            placeholder={placeholder}
            rows={isLandingComposer ? 1 : isExpanded ? 4 : 2}
            size={isLandingComposer ? "compact" : isExpanded ? "expanded" : "compact"}
            value={value}
          />
        </label>
        <div className={styles.controls}>
          <div aria-label={msg("chatComposer.mediaAndSettings")} className={styles.leadingControls} role="group">
            {attachmentsEnabled && <ChatMediaMenu
              disabled={disabled}
              generatedHref={generatedMediaHref}
              labels={{
                chooseGenerated: mediaMenuLabels?.chooseGenerated ?? msg("chatComposer.chooseFromGeneratedFiles"),
                chooseUploaded: mediaMenuLabels?.chooseUploaded ?? msg("chatComposer.chooseFromUploads"),
                menu: mediaMenuLabels?.menu ?? mediaLabel,
                trigger: mediaLabel,
                uploadFile: mediaMenuLabels?.uploadFile ?? msg("chatComposer.uploadFile"),
              }}
              onChooseGenerated={mediaLibraryEnabled ? openGeneratedPicker : onChooseGeneratedMedia}
              onChooseUploaded={mediaLibraryEnabled ? openUploadedPicker : onChooseUploadedMedia}
              onFilesSelected={selectNativeFile}
              uploadedHref={uploadedMediaHref}
            />}
            {leadingControls}
          </div>
          <div className={styles.trailingControls}>
            {additionalControls}
            <ChatSubmitButton
              disabled={!canSubmit || disabled || blocked}
              label={submitLabel}
              onClick={send}
              type="button"
            />
          </div>
        </div>
      </InputSurface>
      {note === undefined ? null : <p className={styles.note}>{note}</p>}
      {preview && previewIndex >= 0 ? <AttachmentPreviewDialog
        items={previewItems}
        selectedIndex={previewIndex}
        onSelect={index => setPreview({ ...preview, id: previewItems[index].id })}
        onClose={() => setPreview(null)}
        returnFocusTo={preview.trigger}
      /> : null}
      {attachmentState.networkNotice ? <ChatUploadNetworkNotice anchorRef={inputSurfaceRef} onClose={attachmentState.dismissNetworkNotice} /> : null}
      {attachmentState.duplicateNotice ? <ChatAttachmentNotice onClose={attachmentState.dismissDuplicateNotice} /> : null}
      {pickerSource === null ? null : (
        <ChatFilePicker
          initialSource={pickerSource}
          onClose={() => setPickerSource(null)}
          onSelect={(selectedAttachment) => {
            add([selectedAttachment]);
            setPickerSource(null);
          }}
        />
      )}
    </>
  );
}
