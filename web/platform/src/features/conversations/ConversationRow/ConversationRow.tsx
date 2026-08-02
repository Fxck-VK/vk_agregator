"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { type JSX, useEffect, useRef, useState } from "react";

import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";
import { parseConversationList, type ConversationItem } from "@/lib/web-api/contracts";

import styles from "./ConversationRow.module.css";

type ConversationRowProps = {
  conversation: ConversationItem;
  isActive: boolean;
};

type RowPanel = "actions" | "rename" | "archive" | null;

export function ConversationRow({ conversation, isActive }: ConversationRowProps): JSX.Element {
  const router = useRouter();
  const title = conversation.title.trim() || ru.conversations.unnamed;
  const conversationPath = `/web/v1/conversations/${conversation.id}` as const;
  const [panel, setPanel] = useState<RowPanel>(null);
  const [nextTitle, setNextTitle] = useState(title);
  const [isPending, setIsPending] = useState(false);
  const [hasError, setHasError] = useState(false);
  const actionToggleRef = useRef<HTMLButtonElement>(null);
  const archiveConfirmRef = useRef<HTMLButtonElement>(null);
  const renameInputRef = useRef<HTMLInputElement>(null);
  const restoreActionFocusRef = useRef(false);
  const rowRef = useRef<HTMLElement>(null);

  const closePanel = (restoreActionFocus = false) => {
    if (isPending) return;
    restoreActionFocusRef.current = restoreActionFocus;
    setHasError(false);
    setPanel(null);
  };

  const openPanel = (nextPanel: Exclude<RowPanel, null>) => {
    if (isPending) return;
    setHasError(false);
    setPanel(nextPanel);
    if (nextPanel === "rename") setNextTitle(title);
  };

  useEffect(() => {
    rowRef.current?.dispatchEvent(
      new CustomEvent("conversation-row-panel-change", { bubbles: true, detail: { open: panel !== null } }),
    );
    if (panel === "rename") renameInputRef.current?.focus();
    if (panel === "archive") archiveConfirmRef.current?.focus();
    if (panel === null && restoreActionFocusRef.current) {
      restoreActionFocusRef.current = false;
      actionToggleRef.current?.focus();
    }
  }, [panel]);

  useEffect(() => {
    if (panel === null) return undefined;

    const closeInnerPanelOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !isPending) {
        restoreActionFocusRef.current = true;
        setHasError(false);
        setPanel(null);
      }
    };
    window.addEventListener("keydown", closeInnerPanelOnEscape);
    return () => window.removeEventListener("keydown", closeInnerPanelOnEscape);
  }, [isPending, panel]);

  const renameConversation = async () => {
    if (isPending) return;
    setHasError(false);
    setIsPending(true);
    try {
      const response = await webBrowserMutation(conversationPath, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ title: nextTitle }),
      });
      if (response.status !== 200) throw new Error("Unable to complete the request.");

      parseConversationList({ items: [await response.json()] });
      setPanel(null);
      router.refresh();
    } catch {
      setHasError(true);
    } finally {
      setIsPending(false);
    }
  };

  const archiveConversation = async () => {
    if (isPending) return;
    setHasError(false);
    setIsPending(true);
    try {
      const response = await webBrowserMutation(conversationPath, { method: "DELETE" });
      if (response.status !== 204) throw new Error("Unable to complete the request.");

      if (isActive) router.replace("/app");
      else router.refresh();
    } catch {
      setHasError(true);
    } finally {
      setIsPending(false);
    }
  };

  return (
    <article className={styles.row} ref={rowRef}>
      <Link aria-current={isActive ? "page" : undefined} className={styles.link} href={`/app/chat/${conversation.id}`}>
        {title}
      </Link>
      <div className={styles.actions} onClick={(event) => event.stopPropagation()}>
        <button
          aria-expanded={panel !== null}
          aria-label={`${ru.conversations.actionsLabel}: ${title}`}
          className={styles.actionToggle}
          disabled={isPending}
          onClick={() => (panel === "actions" ? closePanel(true) : openPanel("actions"))}
          ref={actionToggleRef}
          type="button"
        >
          <span aria-hidden="true">...</span>
        </button>
        {panel !== null ? (
          <div className={styles.panel}>
            {panel === "actions" ? (
              <div className={styles.menu}>
                <button disabled={isPending} onClick={() => openPanel("rename")} type="button">{ru.conversations.renameLabel}</button>
                <button disabled={isPending} onClick={() => openPanel("archive")} type="button">{ru.conversations.archiveLabel}</button>
                <button disabled={isPending} onClick={() => closePanel(true)} type="button">{ru.conversations.cancelLabel}</button>
              </div>
            ) : null}
            {panel === "rename" ? (
              <form className={styles.renameForm} onSubmit={(event) => { event.preventDefault(); void renameConversation(); }}>
                <label>
                  <span className={styles.visuallyHidden}>{ru.conversations.renameInputLabel}</span>
                  <input
                    aria-label={ru.conversations.renameInputLabel}
                    disabled={isPending}
                    onChange={(event) => setNextTitle(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") {
                        event.preventDefault();
                        void renameConversation();
                      }
                    }}
                    ref={renameInputRef}
                    value={nextTitle}
                  />
                </label>
                <div className={styles.formActions}>
                  <button disabled={isPending} type="submit">{isPending ? ru.conversations.renamePending : ru.conversations.renameSubmitLabel}</button>
                  <button disabled={isPending} onClick={() => closePanel(true)} type="button">{ru.conversations.cancelLabel}</button>
                </div>
              </form>
            ) : null}
            {panel === "archive" ? (
              <div className={styles.confirmation}>
                <p>{ru.conversations.archiveConfirmation}</p>
                <div className={styles.formActions}>
                  <button disabled={isPending} onClick={() => void archiveConversation()} ref={archiveConfirmRef} type="button">{isPending ? ru.conversations.archivePending : ru.conversations.archiveConfirmLabel}</button>
                  <button disabled={isPending} onClick={() => closePanel(true)} type="button">{ru.conversations.cancelLabel}</button>
                </div>
              </div>
            ) : null}
            {hasError ? <p className={styles.error} role="alert">{panel === "archive" ? ru.conversations.archiveFailure : ru.conversations.renameFailure}</p> : null}
          </div>
        ) : null}
      </div>
    </article>
  );
}
