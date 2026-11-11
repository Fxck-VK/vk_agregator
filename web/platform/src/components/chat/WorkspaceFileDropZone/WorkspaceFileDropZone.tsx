"use client";

import { useMessages } from "@/i18n/LocaleProvider";


import { createContext, useCallback, useContext, useEffect, useRef, useState, type DragEvent, type ReactNode } from "react";

import Image from "next/image";
import { assetPaths } from "@/assets/asset-paths";

import styles from "./WorkspaceFileDropZone.module.css";

type FileReceiver = (files: File[]) => void;
type RegisteredReceiver = { receive: FileReceiver; accept?: string };
type RegisterReceiver = (receiver: FileReceiver, accept?: string) => () => void;
const FileDropContext = createContext<RegisterReceiver | null>(null);

function hasFiles(transfer: DataTransfer | null) {
  return transfer !== null && Array.from(transfer.types).includes("Files");
}

export function useWorkspaceFileDrop(receiver: FileReceiver, enabled: boolean, accept?: string) {
  const register = useContext(FileDropContext);
  useEffect(() => {
    if (enabled && register) return register(receiver, accept);
  }, [enabled, receiver, register, accept]);
}

export function WorkspaceFileDropZone({ children, className }: { children: ReactNode; className?: string }) {
  const msg = useMessages();
  const rootRef = useRef<HTMLDivElement>(null);
  const receiverRef = useRef<RegisteredReceiver | null>(null);
  const receiversRef = useRef(new Map<symbol, RegisteredReceiver>());
  const dragDepth = useRef(0);
  const [isDragging, setIsDragging] = useState(false);

  const reset = useCallback(() => {
    dragDepth.current = 0;
    setIsDragging(false);
  }, []);

  const register = useCallback<RegisterReceiver>((receive, accept) => {
    const id = Symbol();
    const receiver = { receive, accept };
    receiversRef.current.set(id, receiver);
    receiverRef.current = receiver;
    return () => {
      receiversRef.current.delete(id);
      receiverRef.current = Array.from(receiversRef.current.values()).at(-1) ?? null;
      reset();
    };
  }, [reset]);

  const canReceive = (transfer: DataTransfer) => {
    const receiver = receiverRef.current;
    if (!receiver) return false;
    const types = Array.from(transfer.items ?? []).filter(item => item.kind === "file").map(item => item.type);
    const allowed = receiver.accept?.split(",");
    return !allowed || types.every(type => !type || allowed.includes(type));
  };

  useEffect(() => {
    const preventFileNavigation = (event: globalThis.DragEvent) => {
      if (!hasFiles(event.dataTransfer)) return;
      event.preventDefault();
      if (!rootRef.current?.contains(event.target as Node)) {
        if (event.dataTransfer) event.dataTransfer.dropEffect = "none";
        reset();
      }
    };
    const finishDrop = (event: globalThis.DragEvent) => {
      if (hasFiles(event.dataTransfer)) event.preventDefault();
      reset();
    };
    const cancelFromKeyboard = (event: KeyboardEvent) => {
      if (event.key === "Escape") reset();
    };
    document.addEventListener("dragover", preventFileNavigation);
    document.addEventListener("drop", finishDrop);
    document.addEventListener("keydown", cancelFromKeyboard);
    window.addEventListener("dragend", reset);
    window.addEventListener("blur", reset);
    return () => {
      document.removeEventListener("dragover", preventFileNavigation);
      document.removeEventListener("drop", finishDrop);
      document.removeEventListener("keydown", cancelFromKeyboard);
      window.removeEventListener("dragend", reset);
      window.removeEventListener("blur", reset);
    };
  }, [reset]);

  const dragOver = (event: DragEvent<HTMLDivElement>) => {
    if (!hasFiles(event.dataTransfer) || !event.currentTarget.contains(event.target as Node)) return;
    event.preventDefault();
    const accepts = canReceive(event.dataTransfer);
    event.dataTransfer.dropEffect = accepts ? "copy" : "none";
    setIsDragging(accepts);
  };

  return (
    <FileDropContext.Provider value={register}>
      <div
        className={className}
        data-ui="workspace-file-drop"
        onDragEnter={(event) => {
          if (!hasFiles(event.dataTransfer) || !canReceive(event.dataTransfer) || !event.currentTarget.contains(event.target as Node)) return;
          event.preventDefault();
          dragDepth.current += 1;
          setIsDragging(true);
        }}
        onDragLeave={(event) => {
          if (!hasFiles(event.dataTransfer) || !event.currentTarget.contains(event.target as Node)) return;
          dragDepth.current = Math.max(0, dragDepth.current - 1);
          if (dragDepth.current === 0) reset();
        }}
        onDragOver={dragOver}
        onDrop={(event) => {
          if (!hasFiles(event.dataTransfer) || !event.currentTarget.contains(event.target as Node)) return;
          event.preventDefault();
          const files = Array.from(event.dataTransfer.files);
          reset();
          if (files.length) receiverRef.current?.receive(files);
        }}
        ref={rootRef}
      >
        {children}
        {isDragging ? (
          <div className={styles.overlay} data-testid="workspace-file-drop-overlay" role="status">
            <div className={styles.message}>
              <Image alt="" unoptimized loading="eager" className={styles.artwork} src={assetPaths.images.workspace.fileDrop} width={160} height={120} />
              <strong>{msg("workspaceFileDropZone.dropTheFileToAttachIt")}</strong>
              <p>{msg("workspaceFileDropZone.itWillAppearNextToTheMessage")}</p>
            </div>
          </div>
        ) : null}
      </div>
    </FileDropContext.Provider>
  );
}
