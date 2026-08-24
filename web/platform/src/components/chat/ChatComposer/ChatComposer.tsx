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
import { ChatTextInput } from "@/components/chat/ChatTextInput/ChatTextInput";

import styles from "./ChatComposer.module.css";

export type ChatComposerVariant = "conversation" | "hero" | "newChat" | "workspace";

type ChatComposerProps = {
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
};

const expandedVariants = new Set<ChatComposerVariant>(["hero", "workspace"]);

export function ChatComposer({
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
}: ChatComposerProps) {
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
      <div className={`${styles.surface} ${styles[variant]}`}>
        <label className={styles.field}>
          <span>{label}</span>
          <ChatTextInput
            appearance="composer"
            disabled={disabled}
            onChange={onChange}
            onSend={onSend}
            placeholder={placeholder}
            rows={isExpanded ? 4 : 2}
            size={isExpanded ? "expanded" : "compact"}
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
            <ChatMediaMenu
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
            />
            {leadingControls}
          </div>
          <div className={styles.trailingControls}>
            {additionalControls}
            <button
              aria-label={submitLabel}
              className={styles.submit}
              disabled={!canSubmit || disabled}
              title={submitLabel}
              type="submit"
            >
              <svg aria-hidden="true" focusable="false" viewBox="0 0 24 24">
                <path d="M12 19V5m0 0-6 6m6-6 6 6" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
              </svg>
            </button>
          </div>
        </div>
      </div>
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
