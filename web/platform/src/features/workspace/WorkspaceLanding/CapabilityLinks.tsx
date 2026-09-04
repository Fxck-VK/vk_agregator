import Link from "next/link";

import { capabilityLinks } from "./workspace-home-content";
import styles from "./CapabilityLinks.module.css";

export function CapabilityLinks() {
  return (
    <div className={styles.wrapper}>
      <h3 className={styles.divider}>
        <span>И многое другое</span>
      </h3>
      <nav aria-label="Дополнительные возможности" className={styles.list}>
        {capabilityLinks.map((item) => (
          <Link
            className={styles.link}
            data-testid="workspace-capability-link"
            href={item.href}
            key={item.label}
          >
            <span aria-hidden="true" className={styles.icon} data-testid="workspace-capability-icon" />
            <span>{item.label}</span>
          </Link>
        ))}
      </nav>
    </div>
  );
}
