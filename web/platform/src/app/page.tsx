import type { Metadata } from "next";
import Link from "next/link";
import { connection } from "next/server";

import { ru } from "@/i18n/ru";

import styles from "./page.module.css";

export const metadata: Metadata = {
  title: ru.home.title,
  description: ru.home.description,
};

export default async function HomePage() {
  await connection();

  return (
    <main className={styles.home}>
      <section className={styles.content}>
        <p className={styles.brand}>{ru.brand.name}</p>
        <h1>{ru.home.title}</h1>
        <p className={styles.description}>{ru.home.description}</p>
        <Link className={styles.primaryAction} href="/app">
          {ru.home.primaryAction}
        </Link>
        <p className={styles.supportingText}>{ru.home.supportingText}</p>
      </section>
    </main>
  );
}
