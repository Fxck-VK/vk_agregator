"use client";

import { useLayoutEffect, useState, type RefObject } from "react";
import { createPortal } from "react-dom";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import { PopoverSurface } from "@/components/ui/PopoverPanel/PopoverPanel";
import { useMessages } from "@/i18n/LocaleProvider";
import styles from "./ChatUploadNetworkNotice.module.css";

type Props = { anchorRef: RefObject<HTMLElement | null>; onClose: () => void };
type Layout = { host: HTMLElement; left: number; width: number; inWorkspace: boolean };

export function ChatUploadNetworkNotice({ anchorRef, onClose }: Props) {
  const msg = useMessages();
  const [layout, setLayout] = useState<Layout | null>(null);

  useLayoutEffect(() => {
    const anchor = anchorRef.current;
    if (!anchor) return;
    const workspace = anchor.closest<HTMLElement>('[data-ui="workspace-file-drop"]');
    const host = workspace ?? document.body;
    const updateLayout = () => {
      const bounds = anchor.getBoundingClientRect();
      const left = bounds.left - (workspace?.getBoundingClientRect().left ?? 0);
      setLayout(current => current?.host === host && current.left === left && current.width === bounds.width
        ? current : { host, left, width: bounds.width, inWorkspace: !!workspace });
    };
    updateLayout();
    const observer = new ResizeObserver(updateLayout);
    observer.observe(anchor);
    if (workspace) observer.observe(workspace);
    window.addEventListener("resize", updateLayout);
    return () => { observer.disconnect(); window.removeEventListener("resize", updateLayout); };
  }, [anchorRef]);

  if (!layout) return null;
  return createPortal(
    <PopoverSurface
      animated={false}
      className={styles.notice}
      data-testid="chat-upload-network-notice"
      data-upload-network-notice=""
      data-in-workspace={layout.inWorkspace}
      style={{ left: layout.left, width: layout.width }}
      viewportClassName={styles.content}
    >
      <svg aria-hidden="true" focusable="false" className={styles.warning} data-icon="file-network-error" viewBox="0 0 24 24" fill="none">
        <circle cx="12" cy="12" r="9" stroke="#EF4444" strokeWidth="1.8" />
        <path d="M12 7.5v5" stroke="#EF4444" strokeWidth="1.8" strokeLinecap="round" />
        <circle cx="12" cy="16" r="1" fill="#EF4444" />
      </svg>
      <p role="alert">{msg("useChatAttachments.uploadNetworkError")}</p>
      <ModalCloseButton size="compact" aria-label={msg("chatUploadNetworkNotice.close")} onClick={() => {
        anchorRef.current?.querySelector("textarea")?.focus({ preventScroll: true });
        onClose();
      }} />
    </PopoverSurface>, layout.host,
  );
}
