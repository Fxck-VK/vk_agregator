"use client";

import { BrandName, BrandText } from "@/components/brand/BrandName/BrandName";
import { BrandLogo } from "@/components/brand/BrandLogo/BrandLogo";

import { useMessages, useLocale } from "@/i18n/LocaleProvider";
import { RichMessage } from "@/i18n/RichMessage";

import Image from "next/image";
import Link from "@/i18n/Link";

import { assetPaths } from "@/assets/asset-paths";
import { SubscriptionPlansButton } from "@/components/layout/WorkspaceHeader/SubscriptionPlansButton";
import headerStyles from "@/components/layout/WorkspaceHeader/WorkspaceHeader.module.css";
import { VideoPlayer } from "@/components/media/VideoPlayer/VideoPlayer";
import { FAQ } from "@/components/ui/FAQ/FAQ";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { MasonryGrid } from "@/components/ui/MasonryGrid/MasonryGrid";
import { InspirationExampleCard } from "@/features/inspiration/InspirationExampleCard/InspirationExampleCard";
import { getInspirationExamples } from "@/features/inspiration/inspiration-examples";
import { subscriptionPlanPreview } from "@/features/pricing/subscription-preview";
import { formatPaymentPrice } from "@/features/payments/payments";
import { subscriptionCopy } from "@/features/subscriptions/copy";
import { useLiteOffer } from "@/features/subscriptions/use-lite-offer";
import { useGenerationCatalog } from "@/features/models/GenerationCatalogProvider";

import { FeaturedModels } from "../FeaturedModels/FeaturedModels";
import { WorkspaceHero } from "../WorkspaceHero/WorkspaceHero";

import { CapabilityLinks } from "./CapabilityLinks";
import { getFrequentlyAskedQuestions } from "./workspace-home-content";
import styles from "./WorkspaceLanding.module.css";

type WorkspaceLandingProps = {
  access?: "authenticated" | "guest";
};

export function WorkspaceLanding({ access = "authenticated" }: WorkspaceLandingProps) {
  const msg = useMessages();
  const locale = useLocale();
  const liteOffer = useLiteOffer();
  const { catalog } = useGenerationCatalog();
  const footerModels = [
    { id: "nano_banana_2", name: "Nano Banana 2" },
    { id: "nano_banana_pro", name: "Nano Banana Pro" },
    { id: "gpt_image_2", name: "GPT Image 2" },
    { id: "seedream_4_5", name: "Seedream 4.5" },
  ].filter(model => catalog?.items.some(item => item.id === model.id));
  const copy = subscriptionCopy[locale], trial = subscriptionPlanPreview("lite"), next = subscriptionPlanPreview(trial.next_plan_code);
  const inspirationExamples = getInspirationExamples(locale);
  const frequentlyAskedQuestions = getFrequentlyAskedQuestions(msg);
  const promptExamples = inspirationExamples.filter(example => example.mediaType === "image").slice(0, 6);

  return (
    <div className={styles.page}>
      <div className={styles.main}>
        <section
          aria-labelledby="workspace-home-title"
          className={`${styles.section} ${styles.hero} ${styles.contentFrame}`}
        >
          <div className={styles.heroCopy}>
            <h1 id="workspace-home-title">
              <RichMessage id="workspaceLanding.anEasyStartWith" values={{ highlight: <span>{msg("workspaceLanding.ai")}</span> }} />
            </h1>
            <p>{msg("workspaceLanding.chatsImageGenerationAndUsefulAiTools")}</p>
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
              <span>{msg("workspaceLanding.allAiModels")}</span>
            </Link>
          )} />
        </section>

        <section
          aria-labelledby="workspace-models-title"
          className={`${styles.section} ${styles.contentFrame}`}
        >
          <div className={styles.sectionHeading}>
            <div>
              <h2 id="workspace-models-title">{msg("workspaceLanding.popularAiModels")}</h2>
              <p>{msg("workspaceLanding.exploreSpecificModelsTheirCapabilitiesAndCurrent")}</p>
            </div>
          </div>
          <FeaturedModels />
        </section>

        <section aria-labelledby="workspace-how-title" className={`${styles.section} ${styles.contentFrame}`}>
          <div className={styles.sectionHeading}>
            <div>
              <h2 id="workspace-how-title"><BrandText>{msg("workspaceLanding.howNeirohubWorks")}</BrandText></h2>
              <p>{msg("workspaceLanding.fromYourPromptToAFinishedResult")}</p>
            </div>
          </div>
          <VideoPlayer
            deferPoster
            poster={assetPaths.images.workspace.howItWorksPoster}
            source={{ src: assetPaths.videos.workspace.howItWorks, type: "video/mp4" }}
            title={msg("workspaceLanding.howNeirohubWorks")}
          />
        </section>

        <div className={styles.contentFrame}>
          <CapabilityLinks />
        </div>

        {liteOffer.visible ? <section aria-labelledby="workspace-plan-title" className={`${styles.section} ${styles.contentFrame}`}>
          <div className={styles.planCard}>
            <div
              className={styles.planOffer}
              style={{ backgroundImage: `url("${assetPaths.images.workspace.litePlanBackground}")` }}
            >
              <div className={styles.planHeading}>
                <h2 id="workspace-plan-title">{trial.name}</h2>
                <span aria-hidden="true" className={styles.planDivider} />
                <CreditAmount className={styles.planBalance} value={trial.stars} />
              </div>
              <p className={styles.planPrice}>
                {formatPaymentPrice(trial.amount, locale)} <span>₽ / {trial.period_days} {copy.days}</span>
              </p>
              <SubscriptionPlansButton className={`${headerStyles.tariffButton} ${styles.planButton}`} />
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
                <span><strong>{copy.sharedStars}</strong> {copy.costs}</span>
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
                <span><strong>{trial.period_days} {copy.days}</strong> {copy.expiry}</span>
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
                <span><strong>{copy.next}</strong> {next.name} · {formatPaymentPrice(next.amount, locale)} ₽ · {new Intl.NumberFormat(locale).format(next.stars)} {copy.stars} / {next.period_days} {copy.days}</span>
              </li>
            </ul>
          </div>
        </section>

        : null}
        <section aria-labelledby="workspace-prompts-title" className={`${styles.section} ${styles.contentFrame}`}>
          <div className={styles.sectionHeading}>
            <div>
              <h2 id="workspace-prompts-title">{msg("workspaceLanding.promptLibrary")}</h2>
              <p>{msg("workspaceLanding.promptsForAllKindsOfTasksAnd")}</p>
            </div>
          </div>
          <div data-testid="workspace-prompt-examples">
            <MasonryGrid>
              {promptExamples.map((example, index) => (
                <li key={example.id} data-testid="workspace-prompt-example">
                  <InspirationExampleCard example={example} priority={index === 0} />
                </li>
              ))}
            </MasonryGrid>
          </div>
        </section>

        <section aria-labelledby="workspace-faq-title" className={`${styles.section} ${styles.contentFrame}`}>
          <div className={styles.sectionHeading}>
            <div>
              <h2 id="workspace-faq-title">{msg("workspaceLanding.frequentlyAskedQuestions")}</h2>
            </div>
          </div>
          <FAQ items={frequentlyAskedQuestions} />
        </section>

      </div>

      <footer className={styles.footer}>
        <div className={styles.footerInner}>
          <div className={styles.footerTop}>
            <Link className={styles.footerBrand} href="/app">
              <BrandLogo className={styles.brandMark} size={44} />
              <strong><BrandName /></strong>
            </Link>
            <Link className={styles.footerAgreement} href="/legal/user-agreement" target="_blank" rel="noopener noreferrer">{msg("workspaceLanding.userAgreement")}</Link>
          </div>

          <nav aria-label={msg("workspaceLanding.platformSections")} className={styles.footerColumns}>
            <div className={styles.footerColumn}>
              <strong>{msg("workspaceLanding.aiModels")}</strong>
              {footerModels.map(model => <Link key={model.id} href={`/app/image?model=${model.id}`}>{model.name}</Link>)}
              <Link href="/app/models">{msg("workspaceLanding.allAiModels")}</Link>
            </div>
            <div className={styles.footerColumn}>
              <strong><BrandText>{msg("workspaceLanding.neirohubTools")}</BrandText></strong>
              <Link href="/app/chats">{msg("workspaceLanding.newChat")}</Link>
              <Link href="/app/image">{msg("workspaceLanding.imageGeneration")}</Link>
              <Link href="/app/files">{msg("workspaceLanding.myFiles")}</Link>
              <Link href="/app/inspiration">{msg("workspaceLanding.inspiration")}</Link>
              <Link href="/app/profile">{msg("workspaceLanding.profile")}</Link>
            </div>
          </nav>

          <div className={styles.footerMeta}>
            <span><BrandText>{msg("workspaceLanding.2026NeirohubAllRightsReserved")}</BrandText></span>
            <span>{msg("workspaceLanding.aiModelsInOneWorkspace")}</span>
          </div>
        </div>
      </footer>
    </div>
  );
}
