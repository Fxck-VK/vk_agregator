import Link from "next/link";

import type { PublicDictionary } from "@/i18n/public/dictionary";

import { PageContainer } from "../PageContainer/PageContainer";
import styles from "./PublicFooter.module.css";

type PublicFooterProps = {
  dictionary: PublicDictionary;
};

export function PublicFooter({ dictionary }: PublicFooterProps) {
  return (
    <footer className={styles.footer}>
      <PageContainer className={styles.inner} size="wide">
        <div className={styles.identity}>
          <span aria-hidden="true" className={styles.brandMark}>NH</span>
          <div>
            <strong>{dictionary.brand.name}</strong>
            <p>{dictionary.footer.tagline}</p>
          </div>
        </div>
        <nav aria-label={dictionary.accessibility.primaryNavigation} className={styles.links}>
          <Link href="/">{dictionary.navigation.home}</Link>
          <Link href="/app">{dictionary.footer.workspace}</Link>
        </nav>
      </PageContainer>
    </footer>
  );
}
