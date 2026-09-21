"use client";

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
import { InspirationExampleCard } from "@/features/inspiration/InspirationExampleCard/InspirationExampleCard";
import { getInspirationExamples } from "@/features/inspiration/inspiration-examples";

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
  const inspirationExamples = getInspirationExamples(locale);
  const frequentlyAskedQuestions = getFrequentlyAskedQuestions(msg);
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
              <h2 id="workspace-how-title">{msg("workspaceLanding.howNeirohubWorks")}</h2>
              <p>{msg("workspaceLanding.fromYourPromptToAFinishedResult")}</p>
            </div>
          </div>
          <VideoPlayer
            poster={assetPaths.images.workspace.howItWorksPoster}
            source={{ src: assetPaths.videos.workspace.howItWorks, type: "video/mp4" }}
            title={msg("workspaceLanding.howNeirohubWorks")}
          />
        </section>

        <section aria-labelledby="workspace-capabilities-title" className={`${styles.section} ${styles.contentFrame}`}>
          <div className={styles.sectionHeading}>
            <div>
              <h2 id="workspace-capabilities-title">{msg("workspaceLanding.discoverNewPossibilities")}</h2>
              <p>{msg("workspaceLanding.useDedicatedToolsForTextImagesFiles")}</p>
            </div>
          </div>
          <div className={styles.capabilityMosaic}>
            <Link className={`${styles.capabilityCard} ${styles.capabilityLarge}`} href="/app/image">
              <span className={styles.capabilityVisual}>
                <Image
                  alt={msg("workspaceLanding.aPaperCraneAmongTheClouds")}
                  className={styles.capabilityImage}
                  fill
                  sizes="(max-width: 48rem) 100vw, 23rem"
                  src={assetPaths.images.inspiration.paperCraneCloud}
                />
              </span>
              <span className={styles.capabilityCopy}>
                <strong>{msg("workspaceLanding.createImages")}</strong>
                <small>{msg("workspaceLanding.fromAnIdeaToAResultWith")}</small>
              </span>
            </Link>
            <Link className={styles.capabilityCard} href="/app/chats">
              <span aria-hidden="true" className={`${styles.capabilityVisual} ${styles.capabilityTextVisual}`}>
                <span className={styles.textPreviewInput} />
                <span className={styles.textPreviewAction}>{msg("workspaceLanding.create")}</span>
              </span>
              <span className={styles.capabilityCopy}>
                <strong>{msg("workspaceLanding.workWithText")}</strong>
                <small>{msg("workspaceLanding.questionsPlansIdeasAndOngoingConversations")}</small>
              </span>
            </Link>
            <Link className={styles.capabilityCard} href="/app/files">
              <span aria-hidden="true" className={`${styles.capabilityVisual} ${styles.capabilityFilesVisual}`}>
                <span className={styles.filePreview} />
              </span>
              <span className={styles.capabilityCopy}>
                <strong>{msg("workspaceLanding.storeYourResults")}</strong>
                <small>{msg("workspaceLanding.generatedAndUploadedMaterialsInOnePlace")}</small>
              </span>
            </Link>
          </div>
          <CapabilityLinks />
        </section>

        <section aria-labelledby="workspace-plan-title" className={`${styles.section} ${styles.contentFrame}`}>
          <div className={styles.planCard}>
            <div
              className={styles.planOffer}
              style={{ backgroundImage: `url("${assetPaths.images.workspace.litePlanBackground}")` }}
            >
              <div className={styles.planHeading}>
                <h2 id="workspace-plan-title">Lite</h2>
                <span aria-hidden="true" className={styles.planDivider} />
                <CreditAmount className={styles.planBalance} value={400} />
              </div>
              <p className={styles.planPrice}>
                199 <span>{msg("workspaceLanding.week")}</span>
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
                <span><strong>{msg("workspaceLanding.upTo13ImageGenerations")}</strong> {msg("workspaceLanding.nanoBananaImageGeneratorAndGptImage")}</span>
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
                <span><strong>{msg("workspaceLanding.upTo2VideoGenerations")}</strong> {msg("workspaceLanding.videoGenerationAndAnimationTools")}</span>
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
                <span><strong>{msg("workspaceLanding.accessToPopularAiModels")}</strong> {msg("workspaceLanding.chatgptGeminiClaudeAndMore")}</span>
              </li>
            </ul>
          </div>
        </section>

        <section aria-labelledby="workspace-prompts-title" className={`${styles.section} ${styles.contentFrame}`}>
          <div className={styles.sectionHeading}>
            <div>
              <h2 id="workspace-prompts-title">{msg("workspaceLanding.promptLibrary")}</h2>
              <p>{msg("workspaceLanding.promptsForAllKindsOfTasksAnd")}</p>
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
              <h2 id="workspace-faq-title">{msg("workspaceLanding.frequentlyAskedQuestions")}</h2>
            </div>
          </div>
          <FAQ items={frequentlyAskedQuestions} />
        </section>

        <section aria-labelledby="workspace-community-title" className={`${styles.section} ${styles.communitySection} ${styles.contentFrame}`}>
          <div className={styles.communityCard}>
            <div className={styles.communityCopy}>
              <h2 id="workspace-community-title">{msg("workspaceLanding.followUsOnTelegramAndVk")}</h2>
              <p>{msg("workspaceLanding.stayUpToDateAndWorkFaster")}</p>
              <div aria-label={msg("workspaceLanding.neirohubSocialMedia")} className={styles.communityActions}>
                <span aria-disabled="true" className={styles.communityButton}>Telegram</span>
                <a
                  className={styles.communityButton}
                  href="https://vk.me/neirohub_help"
                  rel="noopener noreferrer"
                  target="_blank"
                >
                  {msg("workspaceLanding.vk")}</a>
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
              <div aria-label={msg("workspaceLanding.socialMedia")} className={styles.footerSocials}>
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
                {msg("workspaceLanding.support")}<span>VK</span>
              </a>
              <span>{msg("workspaceLanding.dailyFrom1000To1900")}</span>
            </div>
            <span className={styles.footerAgreement}>{msg("workspaceLanding.userAgreement")}</span>
          </div>

          <nav aria-label={msg("workspaceLanding.platformSections")} className={styles.footerColumns}>
            <div className={styles.footerColumn}>
              <strong>{msg("workspaceLanding.aiModels")}</strong>
              <Link href="/app/image?model=nano_banana_2">Nano Banana 2</Link>
              <Link href="/app/image?model=nano_banana_pro">Nano Banana Pro</Link>
              <Link href="/app/image?model=gpt_image_2">GPT Image 2</Link>
              <Link href="/app/image?model=seedream_4_5">Seedream 4.5</Link>
              <Link href="/app/models">{msg("workspaceLanding.allAiModels")}</Link>
            </div>
            <div className={styles.footerColumn}>
              <strong>{msg("workspaceLanding.neirohubTools")}</strong>
              <Link href="/app/chats">{msg("workspaceLanding.newChat")}</Link>
              <Link href="/app/image">{msg("workspaceLanding.imageGeneration")}</Link>
              <Link href="/app/files">{msg("workspaceLanding.myFiles")}</Link>
              <Link href="/app/inspiration">{msg("workspaceLanding.inspiration")}</Link>
              <Link href="/app/profile">{msg("workspaceLanding.profile")}</Link>
            </div>
            <div className={styles.footerColumn}>
              <strong>{msg("workspaceLanding.aboutThePlatform")}</strong>
              <Link href="/app">{msg("workspaceLanding.workspace")}</Link>
              <Link href="/app/models">{msg("workspaceLanding.modelCatalog")}</Link>
              <Link href="#workspace-prompts-title">{msg("workspaceLanding.promptLibrary")}</Link>
              <Link href="#workspace-faq-title">{msg("workspaceLanding.frequentlyAskedQuestions")}</Link>
              <a href="https://vk.me/neirohub_help" rel="noopener noreferrer" target="_blank">{msg("workspaceLanding.support311")}</a>
            </div>
          </nav>

          <div className={styles.footerMeta}>
            <span>{msg("workspaceLanding.2026NeirohubAllRightsReserved")}</span>
            <span>{msg("workspaceLanding.aiModelsInOneWorkspace")}</span>
          </div>
        </div>
      </footer>
    </div>
  );
}
