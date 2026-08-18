import Image from "next/image";
import Link from "next/link";

import { WorkspacePrompt } from "../WorkspacePrompt/WorkspacePrompt";

import { capabilityLinks, frequentlyAskedQuestions, primaryTools } from "./workspace-home-content";
import styles from "./WorkspaceLanding.module.css";

type WorkspaceLandingProps = {
  access?: "authenticated" | "guest";
};

export function WorkspaceLanding({ access = "authenticated" }: WorkspaceLandingProps) {
  return (
    <div className={styles.page}>
      <div className={styles.main}>
        <section aria-labelledby="workspace-home-title" className={`${styles.section} ${styles.hero}`}>
          <div className={styles.heroCopy}>
            <h1 id="workspace-home-title">
              Простой старт в мир <span>нейросетей</span>
            </h1>
            <p>Диалоги, генерация изображений и полезные AI-инструменты в одном рабочем пространстве.</p>
          </div>

          <WorkspacePrompt access={access} variant="hero" />

          <nav aria-label="Основные возможности" className={styles.toolRail}>
            {primaryTools.map((tool) => (
              <Link className={styles.toolShortcut} href={tool.href} key={tool.label}>
                <span aria-hidden="true" className={`${styles.toolIcon} ${styles[tool.accent]}`}>
                  {tool.monogram}
                </span>
                <span>{tool.label}</span>
              </Link>
            ))}
            <Link className={styles.allToolsShortcut} href="/app/models">
              <span aria-hidden="true" className={styles.arrowIcon}>→</span>
              <span>Все нейросети</span>
            </Link>
          </nav>
        </section>

        <section aria-labelledby="workspace-news-title" className={styles.section}>
          <div className={styles.sectionHeading}>
            <div>
              <p className={styles.kicker}>Обновления платформы</p>
              <h2 id="workspace-news-title">Новости</h2>
            </div>
          </div>
          <article className={styles.newsCard}>
            <div aria-hidden="true" className={styles.newsVisual}>
              <span>NeiroHub</span>
              <strong>Всё нужное для работы с AI — рядом</strong>
            </div>
            <div className={styles.newsBody}>
              <div>
                <h3>Единое рабочее пространство</h3>
                <p>Начинайте диалог, создавайте изображения и возвращайтесь к результатам без лишних переходов.</p>
              </div>
              <Link className={styles.secondaryButton} href="/app/models">Посмотреть инструменты</Link>
            </div>
          </article>
        </section>

        <section aria-labelledby="workspace-models-title" className={styles.section}>
          <div className={styles.sectionHeading}>
            <div>
              <p className={styles.kicker}>Один аккаунт — разные сценарии</p>
              <h2 id="workspace-models-title">Нейросети для разных задач</h2>
              <p>Выберите подходящий инструмент и продолжайте работу в знакомом интерфейсе.</p>
            </div>
            <Link className={styles.primaryButton} href="/app/models">Все нейросети</Link>
          </div>
          <div className={styles.modelGrid}>
            {primaryTools.map((tool) => (
              <Link className={styles.modelCard} href={tool.href} key={tool.label}>
                <span aria-hidden="true" className={`${styles.modelIcon} ${styles[tool.accent]}`}>
                  {tool.monogram}
                </span>
                <h3>{tool.label}</h3>
                <p>{tool.description}</p>
                <span className={styles.cardLink}>Открыть <span aria-hidden="true">→</span></span>
              </Link>
            ))}
          </div>
        </section>

        <section aria-labelledby="workspace-how-title" className={styles.section}>
          <div className={styles.sectionHeading}>
            <div>
              <p className={styles.kicker}>Коротко о главном</p>
              <h2 id="workspace-how-title">Как работает NeiroHub</h2>
              <p>От запроса до готового результата — в одном понятном сценарии.</p>
            </div>
          </div>
          <div className={styles.howCard}>
            <div aria-hidden="true" className={styles.howGlow} />
            <div className={styles.howContent}>
              <span className={styles.playButton} aria-hidden="true">▶</span>
              <div>
                <strong>Сформулируйте задачу</strong>
                <span>Выберите инструмент, проверьте настройки и получите результат.</span>
              </div>
            </div>
          </div>
        </section>

        <section aria-labelledby="workspace-capabilities-title" className={styles.section}>
          <div className={styles.sectionHeading}>
            <div>
              <p className={styles.kicker}>Не только обычный чат</p>
              <h2 id="workspace-capabilities-title">Откройте новые возможности</h2>
              <p>Используйте отдельные инструменты для текста, изображений, файлов и вдохновения.</p>
            </div>
          </div>
          <div className={styles.capabilityMosaic}>
            <Link className={`${styles.capabilityCard} ${styles.capabilityLarge}`} href="/app/image">
              <Image
                alt="Бумажный журавлик среди облаков"
                className={styles.capabilityImage}
                fill
                sizes="(max-width: 48rem) 100vw, 45vw"
                src="/inspiration/paper-crane-cloud.png"
              />
              <span className={styles.capabilityOverlay}>
                <strong>Создавайте изображения</strong>
                <small>От идеи к результату по текстовому описанию</small>
              </span>
            </Link>
            <Link className={`${styles.capabilityCard} ${styles.capabilityText}`} href="/app/chats">
              <span aria-hidden="true">Aa</span>
              <strong>Работайте с текстом</strong>
              <small>Вопросы, планы, идеи и продолжительные диалоги</small>
            </Link>
            <Link className={`${styles.capabilityCard} ${styles.capabilityFiles}`} href="/app/files">
              <span aria-hidden="true">⌁</span>
              <strong>Храните результаты</strong>
              <small>Созданные и загруженные материалы в одном месте</small>
            </Link>
          </div>
          <nav aria-label="Дополнительные возможности" className={styles.chipList}>
            {capabilityLinks.map((item) => (
              <Link href={item.href} key={item.label}>{item.label}</Link>
            ))}
          </nav>
        </section>

        <section aria-labelledby="workspace-plan-title" className={styles.section}>
          <div className={styles.planCard}>
            <div className={styles.planIntro}>
              <p className={styles.kicker}>Аккаунт и баланс</p>
              <h2 id="workspace-plan-title">Ваш план</h2>
              <p>Проверяйте баланс, способы входа и историю операций в профиле.</p>
              <Link className={styles.lightButton} href="/app/profile">Открыть профиль</Link>
            </div>
            <ul className={styles.planBenefits}>
              <li>История диалогов привязана к аккаунту</li>
              <li>Стоимость задачи видна до запуска</li>
              <li>Результаты доступны в разделе «Мои файлы»</li>
            </ul>
          </div>
        </section>

        <section aria-labelledby="workspace-prompts-title" className={styles.section}>
          <div className={styles.sectionHeading}>
            <div>
              <p className={styles.kicker}>Начните с готовой идеи</p>
              <h2 id="workspace-prompts-title">Библиотека промптов</h2>
              <p>Примеры формулировок для быстрых экспериментов с нейросетями.</p>
            </div>
            <Link className={styles.secondaryButton} href="/app/inspiration">Все идеи</Link>
          </div>
          <Link className={styles.promptFeature} href="/app/inspiration">
            <Image
              alt="Пример изображения из библиотеки промптов"
              className={styles.promptImage}
              height={720}
              src="/inspiration/paper-crane-cloud.png"
              width={540}
            />
            <span>
              <small>Промпт для изображения</small>
              <strong>Воздушная бумажная скульптура среди мягких облаков</strong>
              <em>Посмотреть пример →</em>
            </span>
          </Link>
        </section>

        <section aria-labelledby="workspace-faq-title" className={styles.section}>
          <div className={styles.sectionHeading}>
            <div>
              <p className={styles.kicker}>Помощь по платформе</p>
              <h2 id="workspace-faq-title">Частые вопросы</h2>
            </div>
          </div>
          <div className={styles.faqList}>
            {frequentlyAskedQuestions.map((item) => (
              <details key={item.question}>
                <summary>{item.question}<span aria-hidden="true">⌄</span></summary>
                <p>{item.answer}</p>
              </details>
            ))}
          </div>
        </section>

        <section aria-labelledby="workspace-community-title" className={`${styles.section} ${styles.communitySection}`}>
          <div className={styles.communityCard}>
            <div>
              <p className={styles.kicker}>Идеи и примеры</p>
              <h2 id="workspace-community-title">Сообщество NeiroHub</h2>
              <p>Исследуйте удачные запросы и возвращайтесь к своим результатам в рабочем пространстве.</p>
              <div className={styles.communityActions}>
                <Link className={styles.lightButton} href="/app/inspiration">Перейти во вдохновение</Link>
                <Link className={styles.outlineLightButton} href="/app/files">Открыть мои файлы</Link>
              </div>
            </div>
            <div aria-hidden="true" className={styles.communityArt}>
              <span>NH</span>
              <span>AI</span>
              <span>✦</span>
            </div>
          </div>
        </section>
      </div>

      <footer className={styles.footer}>
        <div className={styles.footerInner}>
          <div className={styles.footerBrand}>
            <span className={styles.brandMark}>NH</span>
            <div>
              <strong>NeiroHub</strong>
              <small>Нейросети в одном рабочем пространстве</small>
            </div>
          </div>
          <nav aria-label="Разделы платформы" className={styles.footerLinks}>
            <div>
              <strong>Инструменты</strong>
              <Link href="/app/chats">Новый чат</Link>
              <Link href="/app/image">Генерация изображений</Link>
              <Link href="/app/models">Все нейросети</Link>
            </div>
            <div>
              <strong>Рабочая область</strong>
              <Link href="/app/files">Мои файлы</Link>
              <Link href="/app/inspiration">Вдохновение</Link>
              <Link href="/app/profile">Профиль</Link>
            </div>
          </nav>
        </div>
      </footer>
    </div>
  );
}
