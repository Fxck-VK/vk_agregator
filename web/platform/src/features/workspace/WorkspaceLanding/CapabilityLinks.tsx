"use client";

import { useMessages } from "@/i18n/LocaleProvider";

import Link from "@/i18n/Link";
import type { CSSProperties } from "react";
import { getCapabilityLinks } from "./workspace-home-content";
import styles from "./CapabilityLinks.module.css";

export function CapabilityLinks() {
  const msg = useMessages();
  const capabilityLinks = getCapabilityLinks(msg);
  return (
    <div className={styles.wrapper}>
      <h3 className={styles.divider}>
        <span>{msg("capabilityLinks.andMuchMore")}</span>
      </h3>
      <nav aria-label={msg("capabilityLinks.moreFeatures")} className={styles.list}>
        {capabilityLinks.map((item) => (
          <Link
            className={styles.link}
            data-testid="workspace-capability-link"
            href={item.href}
            key={item.label}
          >
            <span
              aria-hidden="true"
              className={styles.icon}
              data-testid="workspace-capability-icon"
              style={{ "--capability-icon": `url("${item.icon}")` } as CSSProperties}
            />
            <span>{item.label}</span>
          </Link>
        ))}
      </nav>
    </div>
  );
}
