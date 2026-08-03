"use client";

import { useRef } from "react";

import { ru } from "@/i18n/ru";

import styles from "./FileTypeTabs.module.css";

const fileCategories = ["all", "images", "reports", "presentations", "video", "uploads"] as const;

export type FileCategory = (typeof fileCategories)[number];

type FileTypeTabsProps = {
  onValueChange: (value: FileCategory) => void;
  value: FileCategory;
};

function nextCategoryIndex(currentIndex: number, key: string): number | null {
  if (key === "Home") {
    return 0;
  }
  if (key === "End") {
    return fileCategories.length - 1;
  }
  if (key === "ArrowRight") {
    return (currentIndex + 1) % fileCategories.length;
  }
  if (key === "ArrowLeft") {
    return (currentIndex - 1 + fileCategories.length) % fileCategories.length;
  }
  return null;
}

export function FileTypeTabs({ onValueChange, value }: Readonly<FileTypeTabsProps>) {
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([]);

  return (
    <div aria-label={ru.files.categoryTabsLabel} className={styles.tabList} role="tablist">
      {fileCategories.map((category, index) => {
        const isSelected = category === value;
        return (
          <button
            aria-controls="files-panel"
            aria-selected={isSelected}
            className={styles.tab}
            data-selected={isSelected || undefined}
            id={`files-tab-${category}`}
            key={category}
            onClick={() => onValueChange(category)}
            onKeyDown={(event) => {
              const nextIndex = nextCategoryIndex(index, event.key);
              if (nextIndex === null) {
                return;
              }
              event.preventDefault();
              const nextCategory = fileCategories[nextIndex];
              onValueChange(nextCategory);
              tabRefs.current[nextIndex]?.focus();
            }}
            ref={(element) => {
              tabRefs.current[index] = element;
            }}
            role="tab"
            tabIndex={isSelected ? 0 : -1}
            type="button"
          >
            {ru.files.categories[category]}
          </button>
        );
      })}
    </div>
  );
}
