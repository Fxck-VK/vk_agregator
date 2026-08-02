"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { type JSX, useEffect, useLayoutEffect, useRef, useState } from "react";

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
  const pathname = usePathname();
  const title = conversation.title.trim() || ru.conversations.unnamed;
  const conversationPath = `/web/v1/conversations/${conversation.id}` as const;
  const [panel, setPanel] = useState<RowPanel>(null);
  const [nextTitle, setNextTitle] = useState(title);
  const [isPending, setIsPending] = useState(false);
  const [hasError, setHasError] = useState(false);
  const actionToggleRef = useRef<HTMLButtonElement>(null);
  const archiveConfirmRef = useRef<HTMLButtonElement>(null);
  const renameInputRef = useRef<HTMLInputElement>(null);
  const focusAfterPendingRef = useRef<"rename" | "archive" | null>(null);
  const restoreActionFocusRef = useRef(false);
  const rowRef = useRef<HTMLElement>(null);
  const mountedRef = useRef(true);
  const pathnameRef = useRef(pathname);
  const requestGenerationRef = useRef(0);

  const closePanel = (restoreActionFocus = false) => {
    if (isPending) return;
    restoreActionFocusRef.current = restoreActionFocus;
    setHasError(false);
    setPanel(null);
  };

  const openPanel = (nextPanel: Exclude<RowPanel, null>) => {
    if (isPending) return;
    if (nextPanel === "actions") {
      window.dispatchEvent(
        new CustomEvent("conversation-row-panel-open", { detail: { conversationId: conversation.id } }),
      );
    }
    setHasError(false);
    setPanel(nextPanel);
    if (nextPanel === "rename") setNextTitle(title);
  };

  useEffect(() => {
    if (panel === "rename") renameInputRef.current?.focus();
    if (panel === "archive") archiveConfirmRef.current?.focus();
    if (panel === null && restoreActionFocusRef.current) {
      restoreActionFocusRef.current = false;
      actionToggleRef.current?.focus();
    }
  }, [panel]);

  useEffect(() => {
    const notifySidebar = (open: boolean) => {
      window.dispatchEvent(
        new CustomEvent("conversation-row-panel-change", {
          bubbles: true,
          detail: { conversationId: conversation.id, open },
        }),
      );
    };

    notifySidebar(panel !== null);
    return () => notifySidebar(false);
  }, [conversation.id, panel]);

  useEffect(() => {
    const closeForOtherConversation = (event: Event) => {
      const { conversationId } = (event as CustomEvent<{ conversationId?: unknown }>).detail;
      if (conversationId !== conversation.id) {
        restoreActionFocusRef.current = false;
        setPanel(null);
      }
    };
    window.addEventListener("conversation-row-panel-open", closeForOtherConversation);
    return () => window.removeEventListener("conversation-row-panel-open", closeForOtherConversation);
  }, [conversation.id]);

  useLayoutEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      requestGenerationRef.current += 1;
    };
  }, []);

  useLayoutEffect(() => {
    if (pathnameRef.current === pathname) {
      return;
    }

    pathnameRef.current = pathname;
    requestGenerationRef.current += 1;
    focusAfterPendingRef.current = null;
    setIsPending(false);
    setPanel(null);
  }, [pathname]);

  useEffect(() => {
    if (isPending || focusAfterPendingRef.current === null) return;

    const focusTarget = focusAfterPendingRef.current;
    focusAfterPendingRef.current = null;
    if (focusTarget === "rename") renameInputRef.current?.focus();
    else archiveConfirmRef.current?.focus();
  }, [isPending]);

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
    const requestGeneration = requestGenerationRef.current + 1;
    requestGenerationRef.current = requestGeneration;
    setHasError(false);
    setIsPending(true);
    try {
      const response = await webBrowserMutation(conversationPath, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ title: nextTitle }),
      });
      if (!mountedRef.current || requestGeneration !== requestGenerationRef.current) return;
      if (response.status !== 200) throw new Error("Unable to complete the request.");

      const payload = await response.json();
      if (!mountedRef.current || requestGeneration !== requestGenerationRef.current) return;
      parseConversationList({ items: [payload] });
      setPanel(null);
      actionToggleRef.current?.focus();
      router.refresh();
    } catch {
      if (!mountedRef.current || requestGeneration !== requestGenerationRef.current) return;
      focusAfterPendingRef.current = "rename";
      setHasError(true);
    } finally {
      if (mountedRef.current && requestGeneration === requestGenerationRef.current) setIsPending(false);
    }
  };

  const archiveConversation = async () => {
    if (isPending) return;
    const requestGeneration = requestGenerationRef.current + 1;
    requestGenerationRef.current = requestGeneration;
    setHasError(false);
    setIsPending(true);
    try {
      const response = await webBrowserMutation(conversationPath, { method: "DELETE" });
      if (!mountedRef.current || requestGeneration !== requestGenerationRef.current) return;
      if (response.status !== 204) throw new Error("Unable to complete the request.");

      window.dispatchEvent(new CustomEvent("conversation-row-panel-change", { detail: { conversationId: conversation.id, open: false } }));
      window.dispatchEvent(new CustomEvent("conversation-row-archived", { detail: { conversationId: conversation.id } }));
      setPanel(null);

      if (isActive) {
        router.refresh();
        router.replace("/app");
      } else router.refresh();
    } catch {
      if (!mountedRef.current || requestGeneration !== requestGenerationRef.current) return;
      focusAfterPendingRef.current = "archive";
      setHasError(true);
    } finally {
      if (mountedRef.current && requestGeneration === requestGenerationRef.current) setIsPending(false);
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
                <p id={`archive-confirmation-${conversation.id}`}>{ru.conversations.archiveConfirmation}</p>
                <div className={styles.formActions}>
                  <button aria-describedby={`archive-confirmation-${conversation.id}`} disabled={isPending} onClick={() => void archiveConversation()} ref={archiveConfirmRef} type="button">{isPending ? ru.conversations.archivePending : ru.conversations.archiveConfirmLabel}</button>
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
