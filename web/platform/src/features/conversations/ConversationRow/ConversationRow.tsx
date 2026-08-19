"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { type JSX, useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";

import {
  type WorkspaceConversationItem,
  useOptionalWorkspaceConversationList,
} from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";
import { parseConversationItem } from "@/lib/web-api/contracts";

import { ConversationDeleteDialog } from "./ConversationDeleteDialog";
import { FloatingConversationPanel } from "./FloatingConversationPanel";
import styles from "./ConversationRow.module.css";

type ConversationRowProps = {
  activeConversationId?: string | null;
  conversation: WorkspaceConversationItem;
  isActive: boolean;
  onArchived?: (archive: { conversationId: string; isActive: boolean; sidebarIsActive: boolean; sidebarSession?: number; wasPanelOwner: boolean }) => void;
  onArchiveFailed?: (conversationId: string) => void;
  onArchiveStarted?: (archive: { conversationId: string; isActive: boolean; sidebarIsActive: boolean; sidebarSession?: number; wasPanelOwner: boolean }) => void;
  onPanelClosed?: (conversationId: string) => void;
  onPanelOpened?: (conversationId: string) => void;
  onPendingPanelChange?: (conversationId: string, isPending: boolean, session: number) => void;
  onVisiblePanelChange?: (conversationId: string, closePanel: (() => void) | null, session: number) => void;
  ownsCurrentPanel?: (conversationId: string, session: number | undefined) => boolean;
  sidebarIsActive?: boolean;
  sidebarSession?: number;
};

type RowPanel = "actions" | "rename" | "archive" | null;

export function ConversationRow({
  activeConversationId,
  conversation,
  isActive,
  onArchived,
  onArchiveFailed,
  onArchiveStarted,
  onPanelClosed,
  onPanelOpened,
  onPendingPanelChange,
  onVisiblePanelChange,
  ownsCurrentPanel,
  sidebarIsActive = true,
  sidebarSession,
}: ConversationRowProps): JSX.Element | null {
  const router = useRouter();
  const pathname = usePathname();
  const conversationList = useOptionalWorkspaceConversationList();
  const title = conversation.title.trim() || ru.conversations.unnamed;
  const conversationPath = `/web/v1/conversations/${conversation.id}` as const;
  const [panel, setPanel] = useState<RowPanel>(null);
  const [nextTitle, setNextTitle] = useState(title);
  const [isPending, setIsPending] = useState(false);
  const [isOptimisticallyArchived, setIsOptimisticallyArchived] = useState(false);
  const [hasError, setHasError] = useState(false);
  const [panelSession, setPanelSession] = useState(sidebarSession);
  const actionToggleRef = useRef<HTMLButtonElement>(null);
  const archiveConfirmRef = useRef<HTMLButtonElement>(null);
  const isPendingRef = useRef(false);
  const panelIsVisibleRef = useRef(false);
  const shouldFocusRestoredPanelRef = useRef(false);
  const renameInputRef = useRef<HTMLInputElement>(null);
  const focusAfterPendingRef = useRef<{ kind: "rename" | "archive" | "success"; requestGeneration: number; sidebarSession?: number } | null>(null);
  const restoreActionFocusRef = useRef(false);
  const mountedRef = useRef(true);
  const pathnameRef = useRef(pathname);
  const requestGenerationRef = useRef(0);
  const refreshAfterFocusRef = useRef<number | null>(null);
  const sidebarIsActiveRef = useRef(sidebarIsActive);
  const sidebarSessionRef = useRef(sidebarSession);
  const panelIsVisible = sidebarIsActive
    && panel !== null
    && panelSession === sidebarSession
    && (isPending || activeConversationId === undefined || activeConversationId === conversation.id);

  const isCurrentSidebarSession = (session: number | undefined) => sidebarIsActiveRef.current && sidebarSessionRef.current === session;
  const isCurrentPanelOwner = useCallback(
    (session: number | undefined) => ownsCurrentPanel?.(conversation.id, session) ?? (activeConversationId === undefined || activeConversationId === conversation.id),
    [activeConversationId, conversation.id, ownsCurrentPanel],
  );
  const hasHiddenFailure = hasError
    && panel !== null
    && !panelIsVisible
    && sidebarIsActive
    && panelSession === sidebarSession;

  const closePanel = useCallback((restoreActionFocus = false) => {
    if (isPending) return;
    restoreActionFocusRef.current = restoreActionFocus;
    setHasError(false);
    shouldFocusRestoredPanelRef.current = false;
    setPanel(null);
    onPanelClosed?.(conversation.id);
  }, [conversation.id, isPending, onPanelClosed]);

  const closeVisiblePanel = useCallback(() => {
    if (isPendingRef.current || !panelIsVisibleRef.current) return;

    panelIsVisibleRef.current = false;
    restoreActionFocusRef.current = false;
    shouldFocusRestoredPanelRef.current = false;
    setHasError(false);
    setPanel(null);
    onPanelClosed?.(conversation.id);
  }, [conversation.id, onPanelClosed]);

  const dismissPanel = useCallback(() => closePanel(true), [closePanel]);

  const openPanel = (nextPanel: Exclude<RowPanel, null>) => {
    if (isPending) return;
    if (nextPanel === "actions") {
      setPanelSession(sidebarSession);
      panelIsVisibleRef.current = true;
      onPanelOpened?.(conversation.id);
      if (typeof sidebarSession === "number") {
        onVisiblePanelChange?.(conversation.id, closeVisiblePanel, sidebarSession);
      }
    }
    setHasError(false);
    setPanel(nextPanel);
    if (nextPanel === "rename") setNextTitle(title);
  };

  const restoreFailedPanel = () => {
    if (!hasHiddenFailure || panel === null) {
      openPanel("actions");
      return;
    }

    onPanelOpened?.(conversation.id);
    panelIsVisibleRef.current = true;
    if (typeof sidebarSession === "number") {
      onVisiblePanelChange?.(conversation.id, closeVisiblePanel, sidebarSession);
    }
    shouldFocusRestoredPanelRef.current = true;
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
    if (!shouldFocusRestoredPanelRef.current || !panelIsVisible) return;

    shouldFocusRestoredPanelRef.current = false;
    if (panel === "rename") renameInputRef.current?.focus();
    if (panel === "archive") archiveConfirmRef.current?.focus();
  }, [panel, panelIsVisible]);

  // Sidebar Escape can run after layout work but before passive effects.
  useLayoutEffect(() => {
    if (typeof panelSession !== "number") return;
    onPendingPanelChange?.(conversation.id, isPending && panelIsVisible, panelSession);
    return () => onPendingPanelChange?.(conversation.id, false, panelSession);
  }, [conversation.id, isPending, onPendingPanelChange, panelIsVisible, panelSession]);

  useLayoutEffect(() => {
    isPendingRef.current = isPending;
    panelIsVisibleRef.current = panelIsVisible;
  }, [isPending, panelIsVisible]);

  useLayoutEffect(() => {
    if (typeof panelSession !== "number") return;
    if (!panelIsVisible || isPending) {
      onVisiblePanelChange?.(conversation.id, null, panelSession);
      return () => onVisiblePanelChange?.(conversation.id, null, panelSession);
    }

    onVisiblePanelChange?.(conversation.id, closeVisiblePanel, panelSession);
    return () => onVisiblePanelChange?.(conversation.id, null, panelSession);
  }, [closeVisiblePanel, conversation.id, isPending, onVisiblePanelChange, panelIsVisible, panelSession]);

  useLayoutEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      requestGenerationRef.current += 1;
    };
  }, []);

  useLayoutEffect(() => {
    sidebarIsActiveRef.current = sidebarIsActive;
    sidebarSessionRef.current = sidebarSession;
  }, [sidebarIsActive, sidebarSession]);

  useLayoutEffect(() => {
    if (pathnameRef.current === pathname) {
      return;
    }

    const refreshAfterPathnameChange = refreshAfterFocusRef.current !== null;
    pathnameRef.current = pathname;
    requestGenerationRef.current += 1;
    focusAfterPendingRef.current = null;
    refreshAfterFocusRef.current = null;
    shouldFocusRestoredPanelRef.current = false;
    setHasError(false);
    setIsPending(false);
    setIsOptimisticallyArchived(false);
    setPanel(null);
    if (refreshAfterPathnameChange) router.refresh();
  }, [pathname, router]);

  useEffect(() => {
    if (isPending || focusAfterPendingRef.current === null) return;

    const focusTarget = focusAfterPendingRef.current;
    focusAfterPendingRef.current = null;
    if (!mountedRef.current || focusTarget.requestGeneration !== requestGenerationRef.current) return;
    if (isCurrentSidebarSession(focusTarget.sidebarSession) && isCurrentPanelOwner(focusTarget.sidebarSession)) {
      if (focusTarget.kind === "rename") renameInputRef.current?.focus();
      else if (focusTarget.kind === "archive") archiveConfirmRef.current?.focus();
      else actionToggleRef.current?.focus();
    }
    if (focusTarget.kind === "success") {
      const refreshGeneration = refreshAfterFocusRef.current;
      refreshAfterFocusRef.current = null;
      if (mountedRef.current && refreshGeneration === requestGenerationRef.current) router.refresh();
    }
  }, [isCurrentPanelOwner, isPending, router]);

  const renameConversation = async () => {
    if (isPending) return;
    const requestGeneration = requestGenerationRef.current + 1;
    const mutationSidebarSession = panelSession;
    const previousConversation = conversation;
    requestGenerationRef.current = requestGeneration;
    setHasError(false);
    setIsPending(true);
    conversationList?.updateConversationTitle(conversation.id, nextTitle);
    try {
      const response = await webBrowserMutation(conversationPath, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ title: nextTitle }),
      });
      if (!mountedRef.current) return;
      if (response.status !== 200) throw new Error("Unable to complete the request.");

      const payload = await response.json();
      if (!mountedRef.current) return;
      const canonicalConversation = parseConversationItem(payload);
      if (requestGeneration !== requestGenerationRef.current) {
        router.refresh();
        return;
      }
      conversationList?.replaceConversation(canonicalConversation);
      refreshAfterFocusRef.current = requestGeneration;
      focusAfterPendingRef.current = { kind: "success", requestGeneration, sidebarSession: mutationSidebarSession };
      setPanel(null);
    } catch {
      if (!mountedRef.current || requestGeneration !== requestGenerationRef.current) return;
      conversationList?.replaceConversation(previousConversation);
      if (isCurrentSidebarSession(mutationSidebarSession) && isCurrentPanelOwner(mutationSidebarSession)) {
        onPanelOpened?.(conversation.id);
        focusAfterPendingRef.current = { kind: "rename", requestGeneration, sidebarSession: mutationSidebarSession };
      }
      setHasError(true);
    } finally {
      if (mountedRef.current && requestGeneration === requestGenerationRef.current) setIsPending(false);
    }
  };

  const archiveConversation = async () => {
    if (isPending) return;
    const requestGeneration = requestGenerationRef.current + 1;
    const mutationSidebarSession = panelSession;
    requestGenerationRef.current = requestGeneration;
    setHasError(false);
    setIsPending(true);
    onArchiveStarted?.({
      conversationId: conversation.id,
      isActive,
      sidebarIsActive: isCurrentSidebarSession(mutationSidebarSession),
      sidebarSession: mutationSidebarSession,
      wasPanelOwner: isCurrentPanelOwner(mutationSidebarSession),
    });
    setIsOptimisticallyArchived(true);
    try {
      const response = await webBrowserMutation(conversationPath, { method: "DELETE" });
      if (!mountedRef.current) return;
      if (response.status !== 204) throw new Error("Unable to complete the request.");
      if (requestGeneration !== requestGenerationRef.current) {
        router.refresh();
        return;
      }

      setPanel(null);
      onPanelClosed?.(conversation.id);
      onArchived?.({
        conversationId: conversation.id,
        isActive,
        sidebarIsActive: isCurrentSidebarSession(mutationSidebarSession),
        sidebarSession: mutationSidebarSession,
        wasPanelOwner: isCurrentPanelOwner(mutationSidebarSession),
      });

      if (isActive) {
        router.replace("/app");
      } else router.refresh();
    } catch {
      if (!mountedRef.current || requestGeneration !== requestGenerationRef.current) return;
      onArchiveFailed?.(conversation.id);
      setIsOptimisticallyArchived(false);
      if (isCurrentSidebarSession(mutationSidebarSession) && isCurrentPanelOwner(mutationSidebarSession)) {
        onPanelOpened?.(conversation.id);
        focusAfterPendingRef.current = { kind: "archive", requestGeneration, sidebarSession: mutationSidebarSession };
      }
      setHasError(true);
    } finally {
      if (mountedRef.current && requestGeneration === requestGenerationRef.current) setIsPending(false);
    }
  };

  if (isOptimisticallyArchived) return null;

  return (
    <article className={styles.row}>
      {conversation.isPending ? (
        <span aria-busy="true" className={styles.link}>
          {title}
        </span>
      ) : (
        <Link aria-current={isActive ? "page" : undefined} className={styles.link} href={`/app/chat/${conversation.id}`} id={`sidebar-conversation-${conversation.id}`} prefetch={false}>
          {title}
        </Link>
      )}
      {conversation.isPending ? null : (
        <div
        className={styles.actions}
        onClick={(event) => event.stopPropagation()}
        onKeyDown={(event) => {
          if (event.key === "Escape" && panelIsVisible) {
            event.stopPropagation();
            if (!isPending) closePanel(true);
          }
        }}
      >
        <button
          aria-expanded={panelIsVisible}
          aria-label={`${ru.conversations.actionsLabel}: ${title}`}
          className={styles.actionToggle}
          disabled={isPending}
          onClick={() => {
            if (panelIsVisible && panel === "actions") closePanel(true);
            else if (hasHiddenFailure) restoreFailedPanel();
            else openPanel("actions");
          }}
          ref={actionToggleRef}
          type="button"
        >
          <span aria-hidden="true">...</span>
        </button>
        {panelIsVisible && (panel === "actions" || panel === "rename") ? (
          <FloatingConversationPanel
            anchorRef={actionToggleRef}
            ariaLabel={panel === "actions" ? `${ru.conversations.actionsLabel}: ${title}` : undefined}
            className={styles.floatingPanel}
            dismissible={!isPending}
            onDismiss={dismissPanel}
            placementKey={panel}
            role={panel === "actions" ? "menu" : undefined}
          >
            {panel === "actions" ? (
              <div className={styles.menu}>
                <button disabled={isPending} onClick={() => openPanel("rename")} type="button">
                  <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m4 20 4.3-1 10-10a2.1 2.1 0 0 0-3-3l-10 10L4 20Zm10-12 3 3" /></svg>
                  {ru.conversations.renameLabel}
                </button>
                <button className={styles.deleteMenuItem} disabled={isPending} onClick={() => openPanel("archive")} type="button">
                  <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M4 7h16m-10 4v6m4-6v6M9 4h6l1 3H8l1-3Zm-3 3 1 13h10l1-13" /></svg>
                  {ru.conversations.archiveLabel}
                </button>
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
                  <button disabled={isPending} type="submit">{isPending ? ru.conversations.renamePending : hasError ? ru.conversations.renameRetryLabel : ru.conversations.renameSubmitLabel}</button>
                  <button disabled={isPending} onClick={() => closePanel(true)} type="button">{ru.conversations.cancelLabel}</button>
                </div>
              </form>
            ) : null}
            {hasError ? <p className={styles.error} role="alert">{ru.conversations.renameFailure}</p> : null}
          </FloatingConversationPanel>
        ) : null}
      </div>
      )}
      {panelIsVisible && panel === "archive" ? (
        <ConversationDeleteDialog
          confirmRef={archiveConfirmRef}
          conversationTitle={title}
          errorMessage={hasError ? ru.conversations.archiveFailure : undefined}
          isPending={isPending}
          onCancel={dismissPanel}
          onConfirm={() => void archiveConversation()}
        />
      ) : null}
      {hasHiddenFailure ? <p className={styles.error} role="alert">{panel === "archive" ? ru.conversations.archiveFailure : ru.conversations.renameFailure}</p> : null}
    </article>
  );
}
