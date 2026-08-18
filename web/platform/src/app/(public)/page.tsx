import type { Metadata } from "next";
import { connection } from "next/server";

import { ContentCard } from "@/components/public/ContentCard/ContentCard";
import { PageContainer } from "@/components/public/PageContainer/PageContainer";
import { PrimaryButton } from "@/components/public/PrimaryButton/PrimaryButton";
import { SectionHeading } from "@/components/public/SectionHeading/SectionHeading";
import { ru } from "@/i18n/ru";

import styles from "./page.module.css";

export const metadata: Metadata = {
  title: ru.home.title,
  description: ru.home.description,
};

export default async function HomePage() {
  await connection();

  return (
    <section className={styles.home}>
      <PageContainer size="narrow">
        <ContentCard className={styles.content}>
        <p className={styles.brand}>{ru.brand.name}</p>
        <SectionHeading description={ru.home.description} level={1} title={ru.home.title} />
        <PrimaryButton className={styles.primaryAction} href="/app">{ru.home.primaryAction}</PrimaryButton>
        <p className={styles.supportingText}>{ru.home.supportingText}</p>
        </ContentCard>
      </PageContainer>
    </section>
  );
}
