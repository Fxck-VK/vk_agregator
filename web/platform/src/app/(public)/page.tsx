import type { Metadata } from "next";
import { connection } from "next/server";

import { ContentCard } from "@/components/public/ContentCard/ContentCard";
import { PageContainer } from "@/components/public/PageContainer/PageContainer";
import { PrimaryButton } from "@/components/public/PrimaryButton/PrimaryButton";
import { SectionHeading } from "@/components/public/SectionHeading/SectionHeading";
import { getRequestDictionary, getRequestLocale } from "@/i18n/server";
import { publicPageMetadata } from "@/i18n/seo";
import { LanguageSwitcher } from "@/i18n/LanguageSwitcher";

import styles from "./page.module.css";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getRequestDictionary();
  return { ...publicPageMetadata(await getRequestLocale(), "/"), title: t.home.title, description: t.home.description };
}

export default async function HomePage() {
  await connection();
  const t = await getRequestDictionary();

  return (
    <section className={styles.home}>
      <PageContainer size="narrow">
        <ContentCard className={styles.content}>
        <LanguageSwitcher />
        <p className={styles.brand}>{t.brand.name}</p>
        <SectionHeading description={t.home.description} level={1} title={t.home.title} />
        <PrimaryButton className={styles.primaryAction} href="/app">{t.home.primaryAction}</PrimaryButton>
        <p className={styles.supportingText}>{t.home.supportingText}</p>
        </ContentCard>
      </PageContainer>
    </section>
  );
}
