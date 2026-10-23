"use client";

import { useRef, useState } from "react";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog/ConfirmDialog";
import { useDictionary } from "@/i18n/LocaleProvider";
import { webBrowserMutation } from "@/lib/web-api/browser";
import { useArtifactDeletion } from "./artifact-deletion";

export function ArtifactDeleteButton({ artifactID, className, relatedArtifactIDs = [] }: Readonly<{ artifactID: string; className?: string; relatedArtifactIDs?: readonly string[] }>) {
  const t = useDictionary();
  const { markDeleted } = useArtifactDeletion();
  const [open, setOpen] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const trigger = useRef<HTMLButtonElement>(null);
  const close = () => { setOpen(false); setError(""); };
  const confirm = async () => {
    if (pending) return;
    setPending(true); setError("");
    try {
      // The main file is last, keeping its retry control available if a related
      // format/cover fails. Every request is independently owner-checked and replay-safe.
      const ids = [...new Set(relatedArtifactIDs)].filter(id => id !== artifactID).concat(artifactID);
      for (const id of ids) {
        const response = await webBrowserMutation(`/web/v1/artifacts/${id}`, { method: "DELETE", signal: AbortSignal.timeout(20_000) });
        if (response.status !== 202) { setError(response.status === 409 ? t.files.deleteInUse : t.files.deleteFailure); return; }
        markDeleted(id);
      }
      setOpen(false);
    } catch { setError(t.files.deleteFailure); }
    finally { setPending(false); }
  };
  return <>
    <button aria-label={t.files.deleteLabel} className={className} onClick={() => setOpen(true)} ref={trigger} type="button">
      <svg aria-hidden="true" fill="none" height="24" viewBox="0 0 24 24" width="24"><path d="M4 7h16M9 7V4h6v3m3 0-1 14H7L6 7m4 4v6m4-6v6" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.8" /></svg>
    </button>
    {open ? <ConfirmDialog
      cancelLabel={t.files.deleteCancel}
      confirmLabel={t.files.deleteConfirm}
      description={t.files.deleteDescription}
      errorMessage={error}
      isPending={pending}
      onCancel={close}
      onConfirm={() => void confirm()}
      pendingLabel={t.files.deletePending}
      returnFocusRef={trigger}
      title={t.files.deleteTitle}
    /> : null}
  </>;
}
