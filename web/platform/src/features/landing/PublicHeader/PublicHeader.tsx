"use client";

import Link from "next/link";

import { ru } from "@/i18n/ru";

import { useLandingToolSelection } from "../LandingToolSelection/LandingToolSelection";
import styles from "./PublicHeader.module.css";

type PublicHeaderProps = {
  selectedToolName?: string;
};

export function PublicHeader({ selectedToolName = "NeiroHub Chat" }: PublicHeaderProps) {
  const { selectedTool } = useLandingToolSelection();
  const visibleToolName = selectedToolName === "NeiroHub Chat" ? selectedTool.name : selectedToolName;

  return (
    <header className={styles.header}>
      <div aria-label="Выбранный инструмент" className={styles.modelBadge}>
        <span aria-hidden="true" className={styles.modelIcon}>✦</span>
        <span>{visibleToolName}</span>
        <svg aria-hidden="true" viewBox="0 0 20 20">
          <path d="m6 8 4 4 4-4" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.8" />
        </svg>
      </div>
      <Link className={styles.login} href="/login?next=/app">
        {ru.landing.login}
      </Link>
    </header>
  );
}
