"use client";

import Image from "next/image";
import Link from "next/link";
import { useState } from "react";

import { landingNews } from "../landing-content";
import styles from "./NewsCarousel.module.css";

export function NewsCarousel() {
  const [activeIndex, setActiveIndex] = useState(0);
  const item = landingNews[activeIndex];
  const total = landingNews.length;
  const move = (direction: number) => setActiveIndex((current) => (current + direction + total) % total);

  return (
    <div className={styles.carousel}>
      <div className={styles.heading}>
        <div>
          <p className={styles.eyebrow}>Что нового</p>
          <h2 id="news-title">Новости NeiroHub</h2>
        </div>
        <div className={styles.controls}>
          <span aria-live="polite">{String(activeIndex + 1).padStart(2, "0")} / {String(total).padStart(2, "0")}</span>
          <button aria-label="Предыдущая новость" onClick={() => move(-1)} type="button">←</button>
          <button aria-label="Следующая новость" onClick={() => move(1)} type="button">→</button>
        </div>
      </div>
      <article className={styles.card}>
        <div className={styles.copy}>
          <p className={styles.eyebrow}>Обновление платформы</p>
          <h3>{item.title}</h3>
          <p>{item.description}</p>
          <Link href={item.href}>{item.linkLabel}</Link>
        </div>
        <div className={styles.media}>
          <Image alt={item.imageAlt} fill loading="lazy" sizes="(max-width: 880px) 100vw, 52vw" src={item.imageSrc} />
        </div>
      </article>
    </div>
  );
}
