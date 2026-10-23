"use client";

import { useRef, useState } from "react";
import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import { useDictionary } from "@/i18n/LocaleProvider";
import Link from "@/i18n/Link";
import { modelCards, toolCards } from "./marketing-content";
import { MarketingDialog } from "./MarketingDialog";
import styles from "./MarketingHome.module.css";

export function MarketingSearch({ onClose }: { onClose: () => void }) {
  const { marketing: t } = useDictionary();
  const [query, setQuery] = useState("");
  const results = useRef<HTMLDivElement>(null);
  const entries = [
    ...toolCards.map((card, index) => ({ title: t.toolCards[index].title, description: t.toolCards[index].description, href: card.href })),
    ...modelCards.map(card => ({ title: card.name, description: t.modelsTitle, href: "/app/models" })),
  ].filter(entry => `${entry.title} ${entry.description}`.toLowerCase().includes(query.trim().toLowerCase()));
  return <MarketingDialog title={t.search} onClose={onClose}>
    <form role="search" onSubmit={event => { event.preventDefault(); results.current?.querySelector<HTMLAnchorElement>("a")?.click(); }}>
      <InputSurface className={styles.searchInput}><input type="search" autoComplete="off" aria-label={t.search} placeholder={t.searchPlaceholder} value={query} onChange={event => setQuery(event.target.value)} onKeyDown={event => {
        if (event.key === "ArrowDown") { event.preventDefault(); results.current?.querySelector<HTMLAnchorElement>("a")?.focus(); }
      }} /></InputSurface>
    </form>
    <div className={styles.searchResults} ref={results} onKeyDown={event => {
      if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
      event.preventDefault();
      const links = Array.from(event.currentTarget.querySelectorAll<HTMLAnchorElement>("a"));
      const index = links.findIndex(link => link === document.activeElement);
      links[(index + (event.key === "ArrowDown" ? 1 : -1) + links.length) % links.length]?.focus();
    }}>
      {entries.length ? entries.map(entry => <Link href={entry.href} key={entry.title} className={styles.searchResult}><span><strong>{entry.title}</strong><small>{entry.description}</small></span><span aria-hidden="true">↗</span></Link>) : <p role="status">{t.noResults}</p>}
    </div><p className={styles.searchHint}>{t.searchHint}</p>
  </MarketingDialog>;
}
