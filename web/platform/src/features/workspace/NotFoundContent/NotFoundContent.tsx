"use client";

import Image from "next/image";

import { assetPaths } from "@/assets/asset-paths";
import buttonStyles from "@/components/ui/Button/Button.module.css";
import Link from "@/i18n/Link";
import { useDictionary } from "@/i18n/LocaleProvider";

import styles from "./NotFoundContent.module.css";

export function NotFoundContent() {
  const t = useDictionary().notFound;

  return (
    <section aria-labelledby="not-found-title" className={styles.page}>
      <div className={styles.content}>
        <div aria-hidden="true" className={styles.artwork}>
          <span className={styles.code}>404</span>
          <Image
            alt=""
            className={styles.character}
            height={1086}
            priority
            sizes="(max-width: 480px) 280px, 320px"
            src={assetPaths.illustrations.notFoundCharacter}
            width={1448}
          />
        </div>
        <span className={styles.screenReaderOnly}>{t.codeLabel}</span>
        <h1 className={styles.title} id="not-found-title">{t.title}</h1>
        <Link
          className={`${buttonStyles.button} ${buttonStyles.outline} ${styles.homeLink}`}
          href="/app"
        >
          {t.homeAction}
        </Link>
      </div>
    </section>
  );
}
