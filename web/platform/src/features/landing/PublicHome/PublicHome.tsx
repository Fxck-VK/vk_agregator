import Image from "next/image";
import Link from "next/link";

import { ru } from "@/i18n/ru";

import {
  landingCapabilities,
  landingFaq,
  landingFooterGroups,
  landingModels,
  landingNews,
  landingTools,
} from "../landing-content";
import styles from "./PublicHome.module.css";

export const publicHomeBlockOrder = [
  "hero",
  "quick-tools",
  "trust-strip",
  "news",
  "models",
  "how-it-works",
  "capabilities",
  "use-cases",
  "prompt-library",
  "faq",
  "social",
  "footer",
] as const;

export function PublicHome() {
  const featuredNews = landingNews[0];

  return (
    <div className={styles.home}>
      <section className={styles.hero} data-landing-block="hero">
        <p className={styles.eyebrow}>{ru.landing.heroEyebrow}</p>
        <h1>{ru.landing.heroTitle}</h1>
        <p className={styles.heroDescription}>{ru.landing.heroDescription}</p>
        <div className={styles.composerPreview}>
          <label htmlFor="landing-prompt">Задайте вопрос NeiroHub</label>
          <textarea id="landing-prompt" placeholder="Напишите, что хотите сделать" readOnly rows={4} />
          <Link href="/login?next=/app/chats">Начать чат</Link>
        </div>
      </section>

      <section aria-labelledby="quick-tools-title" className={styles.section} data-landing-block="quick-tools">
        <h2 className={styles.srOnly} id="quick-tools-title">Быстрый выбор инструмента</h2>
        <div className={styles.toolRow}>
          {landingTools.map((tool) => (
            <Link className={styles.toolCard} href={tool.href} key={tool.id}>
              <span aria-hidden="true" className={styles.toolIcon}>{tool.icon}</span>
              <span><strong>{tool.name}</strong><small>{tool.description}</small></span>
            </Link>
          ))}
        </div>
      </section>

      <section aria-label="Преимущества NeiroHub" className={styles.trustStrip} data-landing-block="trust-strip">
        <p><strong>90+</strong><span>нейросетей и инструментов</span></p>
        <p><strong>1</strong><span>аккаунт для всех задач</span></p>
        <p><strong>Сразу</strong><span>видна стоимость запуска</span></p>
      </section>

      <section aria-labelledby="news-title" className={styles.section} data-landing-block="news">
        <div className={styles.sectionHeading}><h2 id="news-title">{ru.landing.newsTitle}</h2><span>01 / {String(landingNews.length).padStart(2, "0")}</span></div>
        <article className={styles.newsCard}>
          <div className={styles.newsCopy}>
            <p className={styles.eyebrow}>Обновление платформы</p>
            <h3>{featuredNews.title}</h3>
            <p>{featuredNews.description}</p>
            <Link href={featuredNews.href}>{featuredNews.linkLabel}</Link>
          </div>
          <Image alt={featuredNews.imageAlt} height={900} src={featuredNews.imageSrc} width={900} />
        </article>
      </section>

      <section aria-labelledby="models-title" className={styles.section} data-landing-block="models">
        <div className={styles.sectionHeadingText}>
          <h2 id="models-title">{ru.landing.modelsTitle}</h2>
          <p>{ru.landing.modelsDescription}</p>
        </div>
        <div className={styles.modelGrid}>
          {landingModels.slice(0, 4).map((model) => (
            <Link className={styles.modelCard} href={model.href} key={model.id}>
              <span aria-hidden="true" className={styles.modelIcon}>{model.icon}</span>
              <h3>{model.name}</h3>
              <p>{model.description}</p>
            </Link>
          ))}
        </div>
        <Link className={styles.secondaryAction} href="/login?next=/app/models">Показать все нейросети</Link>
      </section>

      <section aria-labelledby="how-title" className={styles.section} data-landing-block="how-it-works">
        <h2 id="how-title">{ru.landing.howItWorksTitle}</h2>
        <div className={styles.steps}>
          {[
            ["1", "Выберите инструмент", "Откройте чат, генератор или модель из каталога."],
            ["2", "Опишите задачу", "Введите запрос и при необходимости добавьте настройки."],
            ["3", "Получите результат", "Следите за статусом и находите готовые файлы в библиотеке."],
          ].map(([number, title, description]) => (
            <article key={number}><span>{number}</span><h3>{title}</h3><p>{description}</p></article>
          ))}
        </div>
      </section>

      <section aria-labelledby="capabilities-title" className={styles.section} data-landing-block="capabilities">
        <h2 id="capabilities-title">{ru.landing.capabilitiesTitle}</h2>
        <div className={styles.capabilityGrid}>
          {landingCapabilities.map((capability) => (
            <Link className={styles.capabilityCard} href={capability.href} key={capability.id}>
              <Image alt={capability.imageAlt} height={640} src={capability.imageSrc} width={960} />
              <span><strong>{capability.title}</strong><small>{capability.description}</small></span>
            </Link>
          ))}
        </div>
      </section>

      <section aria-labelledby="use-cases-title" className={styles.section} data-landing-block="use-cases">
        <h2 id="use-cases-title">Нейросети для любой задачи</h2>
        <div className={styles.tagList}>
          {landingTools.slice(1, 6).map((tool) => <Link href={tool.href} key={tool.id}>{tool.name}</Link>)}
        </div>
      </section>

      <section className={styles.promptBanner} data-landing-block="prompt-library">
        <div><p className={styles.eyebrow}>Идеи для старта</p><h2>{ru.landing.promptLibraryTitle}</h2><p>Изучайте примеры, копируйте удачные формулировки и создавайте собственные варианты.</p></div>
        <Link href="/login?next=/app/inspiration">Смотреть примеры</Link>
      </section>

      <section aria-labelledby="faq-title" className={styles.section} data-landing-block="faq">
        <h2 id="faq-title">{ru.landing.faqTitle}</h2>
        <div className={styles.faqList}>
          {landingFaq.map((item) => (
            <details key={item.id}><summary>{item.question}</summary><p>{item.answer}</p></details>
          ))}
        </div>
      </section>

      <section className={styles.social} data-landing-block="social">
        <p className={styles.eyebrow}>NeiroHub</p>
        <h2>{ru.landing.socialTitle}</h2>
        <p>Ссылки на официальные каналы появятся после их подтверждения.</p>
      </section>

      <footer className={styles.footer} data-landing-block="footer">
        <div className={styles.footerBrand}><strong>NeiroHub</strong><p>Нейросети и результаты в одном рабочем пространстве.</p></div>
        {landingFooterGroups.map((group) => (
          <section aria-labelledby={`footer-${group.id}`} key={group.id}>
            <h2 id={`footer-${group.id}`}>{group.title}</h2>
            {group.links.map((link) => <Link href={link.href} key={`${group.id}-${link.href}`}>{link.label}</Link>)}
          </section>
        ))}
      </footer>
    </div>
  );
}
