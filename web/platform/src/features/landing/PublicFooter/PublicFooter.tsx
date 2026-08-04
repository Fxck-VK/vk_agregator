import Link from "next/link";

import { landingFooterGroups } from "../landing-content";
import styles from "./PublicFooter.module.css";

export function PublicFooter() {
  return (
    <footer className={styles.footer}>
      <div className={styles.brand}><strong>NeiroHub</strong><p>Нейросети, рабочие файлы и история задач в одном пространстве.</p></div>
      {landingFooterGroups.map((group) => (
        <section aria-labelledby={`footer-${group.id}`} key={group.id}>
          <h2 id={`footer-${group.id}`}>{group.title}</h2>
          <nav aria-label={group.title}>{group.links.map((link) => <Link href={link.href} key={`${group.id}-${link.href}`}>{link.label}</Link>)}</nav>
        </section>
      ))}
      <div className={styles.bottom}><span>© {new Date().getFullYear()} NeiroHub</span><span>Публичная версия платформы</span></div>
    </footer>
  );
}
