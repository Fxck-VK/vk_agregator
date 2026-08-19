import type { ChangeEventHandler } from "react";

import { ChatMediaMenu, type ChatMediaMenuLabels } from "@/components/chat/ChatMediaMenu/ChatMediaMenu";
import { ChatTextInput } from "@/components/chat/ChatTextInput/ChatTextInput";

import styles from "./ChatComposer.module.css";

export type ChatComposerVariant = "conversation" | "hero" | "newChat" | "workspace";

type ChatComposerProps = {
  canSubmit: boolean;
  disabled: boolean;
  generatedMediaHref?: string;
  label: string;
  mediaLabel: string;
  mediaMenuLabels?: Omit<ChatMediaMenuLabels, "trigger">;
  note?: string;
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
  canSubmit,
  disabled,
  generatedMediaHref,
  label,
  mediaLabel,
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
        <div className={styles.controls}>
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
            onChooseGenerated={onChooseGeneratedMedia}
            onChooseUploaded={onChooseUploadedMedia}
            onFilesSelected={onFilesSelected}
            uploadedHref={uploadedMediaHref}
          />
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
      {note === undefined ? null : <p className={styles.note}>{note}</p>}
    </>
  );
}
