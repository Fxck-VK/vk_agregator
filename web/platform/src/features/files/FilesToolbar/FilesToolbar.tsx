"use client";

import { ru } from "@/i18n/ru";

import styles from "./FilesToolbar.module.css";

export type FileStatusFilter = "all" | "ready" | "in_progress";

type FilesToolbarProps = {
  onQueryChange: (value: string) => void;
  onStatusChange: (value: FileStatusFilter) => void;
  query: string;
  status: FileStatusFilter;
};

export function FilesToolbar({ onQueryChange, onStatusChange, query, status }: Readonly<FilesToolbarProps>) {
  return (
    <div className={styles.toolbar}>
      <label className={styles.search}>
        <span>{ru.files.searchLabel}</span>
        <input
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder={ru.files.searchPlaceholder}
          type="search"
          value={query}
        />
      </label>
      <label className={styles.filter}>
        <span>{ru.files.statusFilterLabel}</span>
        <select onChange={(event) => onStatusChange(event.target.value as FileStatusFilter)} value={status}>
          <option value="all">{ru.files.statusFilterAll}</option>
          <option value="ready">{ru.files.statusFilterReady}</option>
          <option value="in_progress">{ru.files.statusFilterInProgress}</option>
        </select>
      </label>
    </div>
  );
}
