"use client";

import { useDictionary } from "@/i18n/LocaleProvider";

import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";

import styles from "./FileTypeTabs.module.css";

const fileCategories = ["all", "images", "reports", "presentations", "video", "uploads"] as const;

export type FileCategory = (typeof fileCategories)[number];

export function FileTypeTabs({ onValueChange, value }: Readonly<{
  onValueChange: (value: FileCategory) => void;
  value: FileCategory;
}>) {
  const t = useDictionary();
  const items = fileCategories.map((category) => ({
    id: category,
    label: t.files.categories[category],
    elementID: `files-tab-${category}`,
    ariaControls: "files-panel",
  }));

  return (
    <ModeSwitchPanel
      activeID={value}
      ariaLabel={t.files.categoryTabsLabel}
      className={styles.panel}
      fullWidth
      items={items}
      onChange={onValueChange}
      semantics="tabs"
    />
  );
}
