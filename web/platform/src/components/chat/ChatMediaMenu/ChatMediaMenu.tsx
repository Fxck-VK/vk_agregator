"use client";

import Image from "next/image";
import Link from "next/link";
import { useEffect, useId, useRef, useState, type ChangeEvent } from "react";

import { assetPaths } from "@/assets/asset-paths";
import styles from "./ChatMediaMenu.module.css";

export type ChatMediaMenuLabels = {
  chooseGenerated: string;
  chooseUploaded: string;
  menu: string;
  trigger: string;
  uploadFile: string;
};

type ChatMediaMenuProps = {
  disabled?: boolean;
  generatedHref?: string;
  labels: ChatMediaMenuLabels;
  onChooseGenerated?: () => void;
  onChooseUploaded?: () => void;
  onFilesSelected?: (files: File[]) => void;
  uploadedHref?: string;
};

const acceptedMediaTypes = "image/*,video/*,audio/*,application/pdf";

export function ChatMediaMenu({
  disabled = false,
  generatedHref = "/app/files?category=images",
  labels,
  onChooseGenerated,
  onChooseUploaded,
  onFilesSelected,
  uploadedHref = "/app/files?category=uploads",
}: Readonly<ChatMediaMenuProps>) {
  const [isOpen, setIsOpen] = useState(false);
  const menuID = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsOpen(false);
      }
    };
    const closeOnOutsidePointer = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    document.addEventListener("keydown", closeOnEscape);
    document.addEventListener("pointerdown", closeOnOutsidePointer);
    return () => {
      document.removeEventListener("keydown", closeOnEscape);
      document.removeEventListener("pointerdown", closeOnOutsidePointer);
    };
  }, [isOpen]);

  const chooseFile = () => {
    setIsOpen(false);
    inputRef.current?.click();
  };
  const handleFilesSelected = (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? []);
    if (files.length > 0) {
      onFilesSelected?.(files);
    }
    event.target.value = "";
  };
  const chooseUploaded = () => {
    setIsOpen(false);
    onChooseUploaded?.();
  };
  const chooseGenerated = () => {
    setIsOpen(false);
    onChooseGenerated?.();
  };

  return (
    <div className={styles.root} ref={rootRef}>
      <button
        aria-controls={isOpen ? menuID : undefined}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label={labels.trigger}
        className={styles.trigger}
        disabled={disabled}
        onClick={() => setIsOpen((current) => !current)}
        type="button"
      >
        <Image alt="" aria-hidden="true" height={24} src={assetPaths.icons.ui.uploadMedia} unoptimized width={24} />
        <span>{labels.trigger}</span>
      </button>

      <input
        accept={acceptedMediaTypes}
        className={styles.fileInput}
        multiple
        onChange={handleFilesSelected}
        ref={inputRef}
        tabIndex={-1}
        type="file"
      />

      {isOpen ? (
        <div aria-label={labels.menu} className={styles.menu} id={menuID} role="menu">
          <button className={styles.item} onClick={chooseFile} role="menuitem" type="button">
            {labels.uploadFile}
          </button>
          {onChooseUploaded === undefined ? (
            <Link className={styles.item} href={uploadedHref} onClick={() => setIsOpen(false)} role="menuitem">
              {labels.chooseUploaded}
            </Link>
          ) : (
            <button className={styles.item} onClick={chooseUploaded} role="menuitem" type="button">
              {labels.chooseUploaded}
            </button>
          )}
          {onChooseGenerated === undefined ? (
            <Link className={styles.item} href={generatedHref} onClick={() => setIsOpen(false)} role="menuitem">
              {labels.chooseGenerated}
            </Link>
          ) : (
            <button className={styles.item} onClick={chooseGenerated} role="menuitem" type="button">
              {labels.chooseGenerated}
            </button>
          )}
        </div>
      ) : null}
    </div>
  );
}
