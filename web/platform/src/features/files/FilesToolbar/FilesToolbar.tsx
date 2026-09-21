"use client";

import { useDictionary } from "@/i18n/LocaleProvider";

import styles from "./FilesToolbar.module.css";

export type FileStatusFilter = "all" | "ready" | "in_progress";

type FilesToolbarProps = {
  onQueryChange: (value: string) => void;
  onStatusChange: (value: FileStatusFilter) => void;
  query: string;
  status: FileStatusFilter;
};

export function FilesToolbar({ onQueryChange, onStatusChange, query, status }: Readonly<FilesToolbarProps>) {
  const t = useDictionary();
  return (
    <div className={styles.toolbar}>
      <label className={styles.search}>
        <span>{t.files.searchLabel}</span>
        <input
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder={t.files.searchPlaceholder}
          type="search"
          value={query}
        />
      </label>
      <label className={styles.filter}>
        <span>{t.files.statusFilterLabel}</span>
        <select
          onChange={(event) => onStatusChange(event.target.value as FileStatusFilter)}
          value={status}
        >
          <option value="all">{t.files.statusFilterAll}</option>
          <option value="ready">{t.files.statusFilterReady}</option>
          <option value="in_progress">{t.files.statusFilterInProgress}</option>
        </select>
      </label>
    </div>
  );
}
