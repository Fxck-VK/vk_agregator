"use client";

import { type RefObject } from "react";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog/ConfirmDialog";
import { useDictionary } from "@/i18n/LocaleProvider";

type ConversationDeleteDialogProps = {
  cancelRef: RefObject<HTMLButtonElement | null>;
  conversationTitle: string;
  errorMessage?: string;
  isPending: boolean;
  onCancel: () => void;
  onConfirm: () => void;
};

export function ConversationDeleteDialog({
  cancelRef,
  conversationTitle,
  errorMessage,
  isPending,
  onCancel,
  onConfirm,
}: ConversationDeleteDialogProps) {
  const t = useDictionary();
  return (
    <ConfirmDialog
      cancelLabel={t.conversations.cancelLabel}
      cancelRef={cancelRef}
      confirmLabel={t.conversations.archiveConfirmLabel}
      description={t.conversations.archiveConfirmation}
      errorMessage={errorMessage}
      isPending={isPending}
      lead={<>{t.conversations.archiveDialogLead} <strong>{conversationTitle}</strong>.</>}
      onCancel={onCancel}
      onConfirm={onConfirm}
      pendingLabel={t.conversations.archivePending}
      restoreFocus={false}
      title={t.conversations.archiveDialogTitle}
    />
  );
}
