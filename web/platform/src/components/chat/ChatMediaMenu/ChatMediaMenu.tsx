"use client";

import Image from "next/image";
import Link from "next/link";
import { useId, useRef, useState, type ChangeEvent } from "react";

import { assetPaths } from "@/assets/asset-paths";
import { InputControlChip } from "@/components/ui/InputControlChip/InputControlChip";
import { PopoverPanel } from "@/components/ui/PopoverPanel/PopoverPanel";
import selectableStyles from "@/components/ui/selectable-control.module.css";
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
const menuItemClassName = `${selectableStyles.control} ${styles.item}`;

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
  const triggerRef = useRef<HTMLButtonElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

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
    <div className={styles.root}>
      <InputControlChip
        aria-controls={isOpen ? menuID : undefined}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label={labels.trigger}
        className={styles.trigger}
        disabled={disabled}
        onClick={() => setIsOpen((current) => !current)}
        ref={triggerRef}
      >
        <Image alt="" aria-hidden="true" height={24} src={assetPaths.icons.ui.uploadMedia} unoptimized width={24} />
        <span>{labels.trigger}</span>
      </InputControlChip>

      <input
        accept={acceptedMediaTypes}
        className={styles.fileInput}
        multiple
        onChange={handleFilesSelected}
        ref={inputRef}
        tabIndex={-1}
        type="file"
      />

      <PopoverPanel
        anchorRef={triggerRef}
        id={menuID}
        isOpen={isOpen}
        itemVariant="action"
        label={labels.menu}
        onClose={() => setIsOpen(false)}
        role="menu"
        width={320}
      >
        <div className={styles.menu}>
          <button className={menuItemClassName} onClick={chooseFile} role="menuitem" type="button">
            {labels.uploadFile}
          </button>
          {onChooseUploaded === undefined ? (
            <Link className={menuItemClassName} href={uploadedHref} onClick={() => setIsOpen(false)} role="menuitem">
              {labels.chooseUploaded}
            </Link>
          ) : (
            <button className={menuItemClassName} onClick={chooseUploaded} role="menuitem" type="button">
              {labels.chooseUploaded}
            </button>
          )}
          {onChooseGenerated === undefined ? (
            <Link className={menuItemClassName} href={generatedHref} onClick={() => setIsOpen(false)} role="menuitem">
              {labels.chooseGenerated}
            </Link>
          ) : (
            <button className={menuItemClassName} onClick={chooseGenerated} role="menuitem" type="button">
              {labels.chooseGenerated}
            </button>
          )}
        </div>
      </PopoverPanel>
    </div>
  );
}
