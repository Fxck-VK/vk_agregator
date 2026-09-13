import Image from "next/image";
import Link from "next/link";

import { assetPaths } from "@/assets/asset-paths";
import { SubscriptionPlansButton } from "@/components/layout/WorkspaceHeader/SubscriptionPlansButton";
import { VideoPlayer } from "@/components/media/VideoPlayer/VideoPlayer";
import { InspirationExampleCard } from "@/features/inspiration/InspirationExampleCard/InspirationExampleCard";
import { inspirationExamples } from "@/features/inspiration/inspiration-examples";

import { FeaturedModels } from "../FeaturedModels/FeaturedModels";
import { WorkspaceHero } from "../WorkspaceHero/WorkspaceHero";

import { CapabilityLinks } from "./CapabilityLinks";
import { frequentlyAskedQuestions } from "./workspace-home-content";
import styles from "./WorkspaceLanding.module.css";

type WorkspaceLandingProps = {
  access?: "authenticated" | "guest";
};

export function WorkspaceLanding({ access = "authenticated" }: WorkspaceLandingProps) {
  const promptExample = inspirationExamples[0];

  return (
    <div className={styles.page}>
      <div className={styles.main}>
        <section
          aria-labelledby="workspace-home-title"
          className={`${styles.section} ${styles.hero} ${styles.contentFrame}`}
        >
          <div className={styles.heroCopy}>
            <h1 id="workspace-home-title">
              Простой старт в мир <span>нейросетей</span>
            </h1>
            <p>Диалоги, генерация изображений и полезные AI-инструменты в одном рабочем пространстве.</p>
          </div>

          <WorkspaceHero access={access} modelLinksClassName={styles.toolRail} allModelsLink={(
            <Link className={styles.allToolsShortcut} href="/app/models">
              <span aria-hidden="true" className={styles.arrowIcon}>
                <Image
                  alt=""
                  className={styles.modelCountBadge}
                  height={36}
                  src={assetPaths.images.workspace.allModelsBadge}
                  width={48}
                />
                →
              </span>
              <span>Все нейросети</span>
            </Link>
          )} />
        </section>

        <section
          aria-labelledby="workspace-models-title"
          className={`${styles.section} ${styles.contentFrame}`}
        >
          <div className={styles.sectionHeading}>
            <div>
              <h2 id="workspace-models-title">Популярные нейросети</h2>
              <p>Конкретные модели, их возможности и актуальная стоимость запуска.</p>
            </div>
          </div>
          <FeaturedModels />
        </section>

        <section aria-labelledby="workspace-how-title" className={`${styles.section} ${styles.contentFrame}`}>
          <div className={styles.sectionHeading}>
            <div>
              <h2 id="workspace-how-title">Как работает NeiroHub</h2>
              <p>От запроса до готового результата — в одном понятном сценарии.</p>
            </div>
          </div>
          <VideoPlayer
            poster={assetPaths.images.workspace.howItWorksPoster}
            source={{ src: assetPaths.videos.workspace.howItWorks, type: "video/mp4" }}
            title="Как работает NeiroHub"
          />
        </section>

        <section aria-labelledby="workspace-capabilities-title" className={`${styles.section} ${styles.contentFrame}`}>
          <div className={styles.sectionHeading}>
            <div>
              <h2 id="workspace-capabilities-title">Откройте новые возможности</h2>
              <p>Используйте отдельные инструменты для текста, изображений, файлов и вдохновения.</p>
            </div>
          </div>
          <div className={styles.capabilityMosaic}>
            <Link className={`${styles.capabilityCard} ${styles.capabilityLarge}`} href="/app/image">
              <span className={styles.capabilityVisual}>
                <Image
                  alt="Бумажный журавлик среди облаков"
                  className={styles.capabilityImage}
                  fill
                  sizes="(max-width: 48rem) 100vw, 23rem"
                  src={assetPaths.images.inspiration.paperCraneCloud}
                />
              </span>
              <span className={styles.capabilityCopy}>
                <strong>Создавайте изображения</strong>
                <small>От идеи к результату по текстовому описанию</small>
              </span>
            </Link>
            <Link className={styles.capabilityCard} href="/app/chats">
              <span aria-hidden="true" className={`${styles.capabilityVisual} ${styles.capabilityTextVisual}`}>
                <span className={styles.textPreviewInput} />
                <span className={styles.textPreviewAction}>Создать</span>
              </span>
              <span className={styles.capabilityCopy}>
                <strong>Работайте с текстом</strong>
                <small>Вопросы, планы, идеи и продолжительные диалоги</small>
              </span>
            </Link>
            <Link className={styles.capabilityCard} href="/app/files">
              <span aria-hidden="true" className={`${styles.capabilityVisual} ${styles.capabilityFilesVisual}`}>
                <span className={styles.filePreview} />
              </span>
              <span className={styles.capabilityCopy}>
                <strong>Храните результаты</strong>
                <small>Созданные и загруженные материалы в одном месте</small>
              </span>
            </Link>
          </div>
          <CapabilityLinks />
        </section>

        <section aria-labelledby="workspace-plan-title" className={`${styles.section} ${styles.contentFrame}`}>
          <div className={styles.planCard}>
            <div className={styles.planOffer}>
              <div className={styles.planHeading}>
                <h2 id="workspace-plan-title">Lite</h2>
                <span aria-hidden="true" className={styles.planDivider} />
                <span className={styles.planBalance}>
                  <Image
                    alt=""
                    height={24}
                    src={assetPaths.images.credits.star}
                    width={24}
                  />
                  400
                </span>
              </div>
              <p className={styles.planPrice}>
                199 <span>₽/нед</span>
              </p>
              <SubscriptionPlansButton className={styles.planButton} />
            </div>
            <ul className={styles.planBenefits}>
              <li>
                <span aria-hidden="true" className={styles.planBenefitIcon}>
                  <Image
                    alt=""
                    height={20}
                    src="/assets/icons/ui/image-generations-white.svg"
                    unoptimized
                    width={20}
                  />
                </span>
                <span><strong>до 13 генераций изображений:</strong> Nano Banana, Генератор изображений и GPT Image 2</span>
              </li>
              <li>
                <span aria-hidden="true" className={styles.planBenefitIcon}>
                  <Image
                    alt=""
                    height={20}
                    src="/assets/icons/ui/video-generations-white.svg"
                    unoptimized
                    width={20}
                  />
                </span>
                <span><strong>до 2 генераций видео:</strong> генератор видео и инструменты анимации</span>
              </li>
              <li>
                <span aria-hidden="true" className={styles.planBenefitIcon}>
                  <Image
                    alt=""
                    height={20}
                    src="/assets/icons/ui/ai-access-white.svg"
                    unoptimized
                    width={20}
                  />
                </span>
                <span><strong>Доступ к популярным нейросетям:</strong> ChatGPT, Gemini, Claude и другим</span>
              </li>
            </ul>
          </div>
        </section>

        <section aria-labelledby="workspace-prompts-title" className={`${styles.section} ${styles.contentFrame}`}>
          <div className={styles.sectionHeading}>
            <div>
              <h2 id="workspace-prompts-title">Библиотека промптов</h2>
              <p>Собрали промпты для любых задач и идей</p>
            </div>
          </div>
          {promptExample ? (
            <div className={styles.promptExample} data-testid="workspace-prompt-example">
              <InspirationExampleCard
                example={promptExample}
                priority
              />
            </div>
          ) : null}
        </section>

        <section aria-labelledby="workspace-faq-title" className={`${styles.section} ${styles.contentFrame}`}>
          <div className={styles.sectionHeading}>
            <div>
              <h2 id="workspace-faq-title">Частые вопросы</h2>
            </div>
          </div>
          <div className={styles.faqList}>
            {frequentlyAskedQuestions.map((item) => (
              <details key={item.question}>
                <summary>
                  {item.question}
                  <Image
                    alt=""
                    className={styles.faqArrow}
                    height={10}
                    src={assetPaths.icons.ui.faqArrow}
                    width={18}
                  />
                </summary>
                <p>{item.answer}</p>
              </details>
            ))}
          </div>
        </section>

        <section aria-labelledby="workspace-community-title" className={`${styles.section} ${styles.communitySection} ${styles.contentFrame}`}>
          <div className={styles.communityCard}>
            <div className={styles.communityCopy}>
              <h2 id="workspace-community-title">Следи за нами в Telegram и VK</h2>
              <p>Будь в тренде и работай с AI быстрее</p>
              <div aria-label="Социальные сети NeiroHub" className={styles.communityActions}>
                <span aria-disabled="true" className={styles.communityButton}>Telegram</span>
                <a
                  className={styles.communityButton}
                  href="https://vk.me/neirohub_help"
                  rel="noopener noreferrer"
                  target="_blank"
                >
                  ВКонтакте
                </a>
              </div>
            </div>
            <div aria-hidden="true" className={styles.communityVisual}>
              <span className={`${styles.communityPreview} ${styles.communityPreviewLeft}`}>
                <Image
                  alt=""
                  fill
                  sizes="8rem"
                  src={assetPaths.images.inspiration.paperCraneCloud}
                />
              </span>
              <span className={styles.socialPhone}>
                <span className={styles.phoneSpeaker} />
                <span className={styles.qrPlaceholder}>
                  <span className={styles.qrMarker} />
                </span>
                <span className={styles.phoneLabel}>NeiroHub</span>
              </span>
              <span className={`${styles.communityPreview} ${styles.communityPreviewRight}`}>
                <Image
                  alt=""
                  fill
                  sizes="8rem"
                  src={assetPaths.images.workspace.howItWorksPoster}
                />
              </span>
            </div>
          </div>
        </section>
      </div>

      <footer className={styles.footer}>
        <div className={styles.footerInner}>
          <div className={styles.footerTop}>
            <div className={styles.footerBrandGroup}>
              <Link className={styles.footerBrand} href="/app">
                <span className={styles.brandMark}>NH</span>
                <strong>NeiroHub</strong>
              </Link>
              <div aria-label="Социальные сети" className={styles.footerSocials}>
                <a href="https://vk.me/neirohub_help" rel="noopener noreferrer" target="_blank">VK</a>
              </div>
            </div>
            <div className={styles.footerSupport}>
              <a
                className={styles.footerSupportButton}
                href="https://vk.me/neirohub_help"
                rel="noopener noreferrer"
                target="_blank"
              >
                Служба поддержки <span>VK</span>
              </a>
              <span>с 10:00 до 19:00 каждый день</span>
            </div>
            <span className={styles.footerAgreement}>Пользовательское соглашение</span>
          </div>

          <nav aria-label="Разделы платформы" className={styles.footerColumns}>
            <div className={styles.footerColumn}>
              <strong>Нейросети</strong>
              <Link href="/app/image?model=nano-banana-2">Nano Banana 2</Link>
              <Link href="/app/image?model=nano-banana-pro">Nano Banana Pro</Link>
              <Link href="/app/image?model=gpt-image-2">GPT Image 2</Link>
              <Link href="/app/image?model=seedream-4-5">Seedream 4.5</Link>
              <Link href="/app/models">Все нейросети</Link>
            </div>
            <div className={styles.footerColumn}>
              <strong>Инструменты NeiroHub</strong>
              <Link href="/app/chats">Новый чат</Link>
              <Link href="/app/image">Генерация изображений</Link>
              <Link href="/app/files">Мои файлы</Link>
              <Link href="/app/inspiration">Вдохновение</Link>
              <Link href="/app/profile">Профиль</Link>
            </div>
            <div className={styles.footerColumn}>
              <strong>О платформе</strong>
              <Link href="/app">Рабочее пространство</Link>
              <Link href="/app/models">Каталог моделей</Link>
              <Link href="#workspace-prompts-title">Библиотека промптов</Link>
              <Link href="#workspace-faq-title">Частые вопросы</Link>
              <a href="https://vk.me/neirohub_help" rel="noopener noreferrer" target="_blank">Поддержка</a>
            </div>
          </nav>

          <div className={styles.footerMeta}>
            <span>© 2026 NeiroHub. Все права защищены.</span>
            <span>Нейросети в одном рабочем пространстве</span>
          </div>
        </div>
      </footer>
    </div>
  );
}
