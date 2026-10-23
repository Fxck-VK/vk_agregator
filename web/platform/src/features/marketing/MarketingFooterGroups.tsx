"use client";

import { useState, useSyncExternalStore } from "react";
import { useDictionary } from "@/i18n/LocaleProvider";
import Link from "@/i18n/Link";
import { footerDestinations } from "./marketing-content";
import styles from "./MarketingHome.module.css";

function subscribe(callback: () => void) {
  const query = window.matchMedia?.("(max-width: 600px)");
  query?.addEventListener("change", callback);
  return () => query?.removeEventListener("change", callback);
}
function snapshot() { return window.matchMedia?.("(max-width: 600px)").matches ?? false; }
function serverSnapshot() { return false; }

export function MarketingFooterGroups() {
  const { marketing: t } = useDictionary();
  const compact = useSyncExternalStore(subscribe, snapshot, serverSnapshot);
  const [expanded, setExpanded] = useState([true, false, false]);
  return t.footerGroups.map((group, index) => {
    const first = index === 0 ? 0 : index === 1 ? 5 : 9;
    const last = index === 0 ? 5 : index === 1 ? 9 : 11;
    const open = !compact || expanded[index];
    return <div key={group} className={styles.footerGroup}>
      <h3><button type="button" disabled={!compact} aria-expanded={open} aria-controls={`footer-group-${index}`} onClick={() => setExpanded(current => current.map((value, position) => position === index ? !value : value))}>{group}<span aria-hidden="true">{open ? "−" : "+"}</span></button></h3>
      <div id={`footer-group-${index}`} hidden={!open}>{t.footerLinks.slice(first, last).map((label, offset) => {
        const href = footerDestinations[first + offset];
        return href.startsWith("mailto:") ? <a href={href} key={label}>{label}</a> : <Link href={href} key={label}>{label}</Link>;
      })}</div>
    </div>;
  });
}
