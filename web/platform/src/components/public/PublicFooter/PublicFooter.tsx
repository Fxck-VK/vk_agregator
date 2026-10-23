import { BrandName } from "@/components/brand/BrandName/BrandName";
import { BrandLogo } from "@/components/brand/BrandLogo/BrandLogo";
import Link from "@/i18n/Link";
import { LegalLinks } from "@/features/legal/LegalLinks";

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
          <BrandLogo className={styles.brandMark} size={44} />
          <div>
            <strong><BrandName /></strong>
            <p>{dictionary.footer.tagline}</p>
          </div>
        </div>
        <nav aria-label={dictionary.accessibility.primaryNavigation} className={styles.links}>
          <Link href="/">{dictionary.navigation.home}</Link>
          <Link href="/app">{dictionary.footer.workspace}</Link>
        </nav>
        <LegalLinks />
      </PageContainer>
    </footer>
  );
}
