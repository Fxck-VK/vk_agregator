"use client";

/* eslint-disable @next/next/no-img-element */

import { useState, type ChangeEventHandler, type ReactNode } from "react";

import {
  attachmentFromFile,
  ChatFilePicker,
  type ChatFileSource,
  type ChatMediaAttachment,
} from "@/components/chat/ChatFilePicker/ChatFilePicker";
import { ChatMediaMenu, type ChatMediaMenuLabels } from "@/components/chat/ChatMediaMenu/ChatMediaMenu";
import { ChatSubmitButton } from "@/components/chat/ChatSubmitButton/ChatSubmitButton";
import { ChatTextInput } from "@/components/chat/ChatTextInput/ChatTextInput";
import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import styles from "./ChatComposer.module.css";

export type ChatComposerVariant = "conversation" | "hero" | "newChat" | "workspace";

type ChatComposerProps = {
  attachmentsEnabled?: boolean;
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
  const isLandingComposer = variant === "hero" || variant === "newChat";
  const isExpanded = expandedVariants.has(variant);
  const [attachment, setAttachment] = useState<ChatMediaAttachment | null>(null);
  const [pickerSource, setPickerSource] = useState<Exclude<ChatFileSource, "all"> | null>(null);
  const selectNativeFile = (files: File[]) => {
    const [file] = files;
    if (file !== undefined) {
      setAttachment(attachmentFromFile(file));
    }
    onFilesSelected?.(files);
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
      <InputSurface className={`${styles.surface} ${styles[variant]}`} data-wrap-leading-controls={wrapLeadingControls || undefined}>
        <label className={styles.field}>
          <span>{label}</span>
          <ChatTextInput
            appearance="composer"
            disabled={disabled}
            onChange={onChange}
            onSend={onSend}
            placeholder={placeholder}
            rows={isLandingComposer ? 1 : isExpanded ? 4 : 2}
            size={isLandingComposer ? "compact" : isExpanded ? "expanded" : "compact"}
            value={value}
          />
        </label>
        {attachment === null ? null : (
          <div className={styles.attachment}>
            {attachment.previewUrl === undefined ? (
              <span aria-hidden="true" className={styles.fileIcon}>+</span>
            ) : (
              <img alt="" src={attachment.previewUrl} />
            )}
            <span title={attachment.name}>{attachment.name}</span>
            <button
              aria-label={`Убрать ${attachment.name}`}
              onClick={() => setAttachment(null)}
              type="button"
            >
              ×
            </button>
          </div>
        )}
        <div className={styles.controls}>
          <div aria-label="Медиа и настройки" className={styles.leadingControls} role="group">
            {attachmentsEnabled && <ChatMediaMenu
              disabled={disabled}
              generatedHref={generatedMediaHref}
              labels={{
                chooseGenerated: mediaMenuLabels?.chooseGenerated ?? "Выбрать из сгенерированных",
                chooseUploaded: mediaMenuLabels?.chooseUploaded ?? "Выбрать из загруженных",
                menu: mediaMenuLabels?.menu ?? mediaLabel,
                trigger: mediaLabel,
                uploadFile: mediaMenuLabels?.uploadFile ?? "Загрузить файл",
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
              disabled={!canSubmit || disabled}
              label={submitLabel}
              type="submit"
            />
          </div>
        </div>
      </InputSurface>
      {note === undefined ? null : <p className={styles.note}>{note}</p>}
      {pickerSource === null ? null : (
        <ChatFilePicker
          initialSource={pickerSource}
          onClose={() => setPickerSource(null)}
          onSelect={(selectedAttachment) => {
            setAttachment(selectedAttachment);
            setPickerSource(null);
          }}
        />
      )}
    </>
  );
}
