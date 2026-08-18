import type { ReactNode } from "react";

import type { PublicDictionary } from "@/i18n/public/dictionary";

import { PublicFooter } from "../PublicFooter/PublicFooter";
import { PublicHeader, type PublicNavigationItem } from "../PublicHeader/PublicHeader";
import styles from "./PublicShell.module.css";

type PublicShellProps = {
  children: ReactNode;
  dictionary: PublicDictionary;
  navigationItems?: readonly PublicNavigationItem[];
};

export function PublicShell({ children, dictionary, navigationItems }: PublicShellProps) {
  return (
    <div className={styles.shell}>
      <a className={styles.skipLink} href="#public-main">{dictionary.accessibility.skipToContent}</a>
      <PublicHeader dictionary={dictionary} navigationItems={navigationItems} />
      <main className={styles.main} id="public-main">{children}</main>
      <PublicFooter dictionary={dictionary} />
    </div>
  );
}
