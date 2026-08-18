import Link from "next/link";

import type { PublicDictionary } from "@/i18n/public/dictionary";

import { PageContainer } from "../PageContainer/PageContainer";
import { PrimaryButton } from "../PrimaryButton/PrimaryButton";
import { PublicThemeSwitcher } from "../PublicThemeSwitcher/PublicThemeSwitcher";
import styles from "./PublicHeader.module.css";

export type PublicNavigationItem = {
  href: string;
  label: string;
};

type PublicHeaderProps = {
  dictionary: PublicDictionary;
  navigationItems?: readonly PublicNavigationItem[];
};

export function PublicHeader({ dictionary, navigationItems = [] }: PublicHeaderProps) {
  return (
    <header className={styles.header}>
      <PageContainer className={styles.inner} size="wide">
        <Link className={styles.brand} href="/">
          <span aria-hidden="true" className={styles.brandMark}>NH</span>
          <span>{dictionary.brand.name}</span>
        </Link>

        {navigationItems.length > 0 ? (
          <nav aria-label={dictionary.accessibility.primaryNavigation} className={styles.navigation}>
            {navigationItems.map((item) => (
              <Link href={item.href} key={item.href}>{item.label}</Link>
            ))}
          </nav>
        ) : <span className={styles.spacer} />}

        <div className={styles.actions}>
          <PublicThemeSwitcher
            labels={{
              dark: dictionary.theme.dark,
              group: dictionary.theme.label,
              light: dictionary.theme.light,
              system: dictionary.theme.system,
            }}
          />
          <PrimaryButton href="/app">{dictionary.navigation.openWorkspace}</PrimaryButton>
        </div>
      </PageContainer>
    </header>
  );
}
