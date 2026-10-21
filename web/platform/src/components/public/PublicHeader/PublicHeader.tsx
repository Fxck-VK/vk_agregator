import { BrandName } from "@/components/brand/BrandName/BrandName";
import { BrandLogo } from "@/components/brand/BrandLogo/BrandLogo";
import Link from "@/i18n/Link";

import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";
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
          <BrandLogo className={styles.brandMark} size={32} />
          <span><BrandName /></span>
        </Link>

        {navigationItems.length > 0 ? (
          <ScrollArea
            className={styles.navigationScroll}
            orientation="horizontal"
            viewportAs="nav"
            viewportClassName={styles.navigation}
            viewportProps={{ "aria-label": dictionary.accessibility.primaryNavigation }}
          >
            {navigationItems.map((item) => (
              <Link href={item.href} key={item.href}>{item.label}</Link>
            ))}
          </ScrollArea>
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
