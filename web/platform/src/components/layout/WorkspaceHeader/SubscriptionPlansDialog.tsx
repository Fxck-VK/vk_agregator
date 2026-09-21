"use client";

import { useMessages } from "@/i18n/LocaleProvider";

import { useEffect, useRef, useState } from "react";

import { assetPaths } from "@/assets/asset-paths";
import { AssetIcon } from "@/components/icons/AssetIcon";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";

import { PromoCodeDialog } from "./PromoCodeDialog";
import styles from "./SubscriptionPlansDialog.module.css";

type SubscriptionPlansDialogProps = {
  onClose: () => void;
};

type Plan = {
  name: string;
  price: string;
  period: string;
  previousPrice?: string;
  discount?: string;
  description: string;
  creditPeriod: string;
  credits: string;
  features: {
    icon: "imageGeneration" | "videoGeneration" | "aiModels" | "presentationGeneration";
    text: string;
  }[];
  popular?: boolean;
};

function PlanCard({ plan }: { plan: Plan }) {
  const msg = useMessages();
  return (
    <article className={`${styles.planCard} ${plan.popular ? styles.popularPlan : ""}`}>
      {plan.popular ? <span className={styles.popularLabel}>{msg("subscriptionPlansDialog.popular")}</span> : null}
      {plan.discount ? <span className={styles.discount}>{plan.discount}</span> : null}

      <div className={styles.planSummary}>
        <h3>{plan.name}</h3>
        <p className={styles.price}>
          <strong>{plan.price}</strong> <span>{plan.period}</span>{" "}
          {plan.previousPrice ? <del>{plan.previousPrice}</del> : null}
        </p>
        <p className={styles.description}>{plan.description}</p>
        <button className={styles.subscribeButton} type="button">
          {msg("subscriptionPlansDialog.activateSubscription")}</button>
      </div>

      <div className={styles.creditRow}>
        <span>{plan.creditPeriod}:</span>
        <strong><CreditAmount className={styles.creditAmount} value={Number(plan.credits)} /></strong>
      </div>

      <ul className={styles.featureList}>
        {plan.features.map((feature) => (
          <li key={feature.icon}>
            <AssetIcon className={styles.featureIcon} iconName={feature.icon} source={assetPaths.icons.plans[feature.icon]} />
            <span>{feature.text}</span>
          </li>
        ))}
      </ul>
    </article>
  );
}

export function SubscriptionPlansDialog({ onClose }: SubscriptionPlansDialogProps) {
  const msg = useMessages();
  const plans: Plan[] = [
    {
      name: "Lite",
      price: "199",
      period: msg("subscriptionPlansDialog.week"),
      description: msg("subscriptionPlansDialog.trialPlan"),
      creditPeriod: msg("subscriptionPlansDialog.oneTime"),
      credits: "400",
      features: [
        { icon: "imageGeneration", text: msg("subscriptionPlansDialog.upTo13ImageGenerationsNanoBanana") },
        { icon: "videoGeneration", text: msg("subscriptionPlansDialog.upTo2VideoGenerationsVideoGenerator") },
        { icon: "aiModels", text: msg("subscriptionPlansDialog.accessToPopularAiModelsChatgptGemini") },
      ],
    },
    {
      name: "Start+",
      price: "549",
      period: msg("subscriptionPlansDialog.month"),
      previousPrice: "632",
      discount: "−15%",
      description: msg("subscriptionPlansDialog.aGreatPlaceToStart"),
      creditPeriod: msg("subscriptionPlansDialog.monthly"),
      credits: "1250",
      features: [
        { icon: "imageGeneration", text: msg("subscriptionPlansDialog.upTo41ImageGenerationsWithPopular") },
        { icon: "videoGeneration", text: msg("subscriptionPlansDialog.upTo15VideoGenerations") },
        { icon: "aiModels", text: msg("subscriptionPlansDialog.accessToChatgptGeminiClaudeSunoKimi") },
        { icon: "presentationGeneration", text: msg("subscriptionPlansDialog.4PresentationGenerations") },
      ],
    },
    {
      name: "Pro",
      price: "999",
      period: msg("subscriptionPlansDialog.month"),
      previousPrice: "1200",
      discount: "−20%",
      description: msg("subscriptionPlansDialog.aBalancedChoice"),
      creditPeriod: msg("subscriptionPlansDialog.monthly"),
      credits: "2250",
      features: [
        { icon: "imageGeneration", text: msg("subscriptionPlansDialog.upTo75ImageGenerationsWithPopular") },
        { icon: "videoGeneration", text: msg("subscriptionPlansDialog.upTo15VideoGenerations") },
        { icon: "aiModels", text: msg("subscriptionPlansDialog.accessToAllCoreAiModels") },
        { icon: "presentationGeneration", text: msg("subscriptionPlansDialog.7PresentationGenerations") },
      ],
    },
    {
      name: "Ultima",
      price: "1999",
      period: msg("subscriptionPlansDialog.month"),
      previousPrice: "2600",
      discount: "−30%",
      description: msg("subscriptionPlansDialog.morePossibilities"),
      creditPeriod: msg("subscriptionPlansDialog.monthly"),
      credits: "4800",
      features: [
        { icon: "imageGeneration", text: msg("subscriptionPlansDialog.upTo160ImageGenerations") },
        { icon: "videoGeneration", text: msg("subscriptionPlansDialog.upTo32VideoGenerations") },
        { icon: "aiModels", text: msg("subscriptionPlansDialog.extendedAiModelAccess") },
        { icon: "presentationGeneration", text: msg("subscriptionPlansDialog.16PresentationGenerations") },
      ],
      popular: true,
    },
    {
      name: "Elite",
      price: "4999",
      period: msg("subscriptionPlansDialog.month"),
      previousPrice: "7000",
      discount: "−40%",
      description: msg("subscriptionPlansDialog.maximumValue"),
      creditPeriod: msg("subscriptionPlansDialog.monthly"),
      credits: "12000",
      features: [
        { icon: "imageGeneration", text: msg("subscriptionPlansDialog.upTo408ImageGenerations") },
        { icon: "videoGeneration", text: msg("subscriptionPlansDialog.upTo62VideoGenerations") },
        { icon: "aiModels", text: msg("subscriptionPlansDialog.maximumLimitsAcrossAllAiModels") },
        { icon: "presentationGeneration", text: msg("subscriptionPlansDialog.41PresentationGenerations") },
      ],
    },
  ];
  const teamFeatures = [
    [msg("subscriptionPlansDialog.customLimits"), msg("subscriptionPlansDialog.flexibleSettingsForYourNeeds"), "customLimits"],
    [msg("subscriptionPlansDialog.prioritySpeed"), msg("subscriptionPlansDialog.dedicatedCapacityWithoutQueues"), "prioritySpeed"],
    [msg("subscriptionPlansDialog.teamCollaboration"), msg("subscriptionPlansDialog.sharedWorkspacesAndAccessManagement"), "teamwork"],
    [msg("subscriptionPlansDialog.personalSupport"), msg("subscriptionPlansDialog.anAccountManagerAndFastSupport"), "personalSupport"],
  ] as const;

  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const promoButtonRef = useRef<HTMLButtonElement>(null);
  const [promoOpen, setPromoOpen] = useState(false);

  useEffect(() => {
    const previouslyFocusedElement = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;
    closeButtonRef.current?.focus();

    return () => {
      previouslyFocusedElement?.focus();
    };
  }, []);

  return (
    <>
      <ModalBackdrop closeOnBackdropClick={!promoOpen} closeOnEscape={!promoOpen} onClose={onClose} testId="subscription-plans-backdrop">
        {(requestClose) => <section
          aria-hidden={promoOpen || undefined}
          aria-labelledby="subscription-plans-title"
          aria-modal={!promoOpen}
          className={styles.dialog}
          inert={promoOpen}
          role="dialog"
        >
          <ScrollArea className={styles.scrollArea}>
            <header className={styles.dialogHeader}>
              <div>
                <h2 id="subscription-plans-title">{msg("subscriptionPlansDialog.unlockMoreWithASubscription")}</h2>
              </div>
              <ModalCloseButton
                aria-label={msg("subscriptionPlansDialog.closePlans")}
                className={styles.closeButtonPlacement}
                onClick={requestClose}
                ref={closeButtonRef}
              />
              <button aria-haspopup="dialog" aria-expanded={promoOpen} className={styles.promoButton} onClick={() => setPromoOpen(true)} ref={promoButtonRef} type="button">{msg("subscriptionPlansDialog.activatePromoCode")}</button>
            </header>

            <div className={styles.plansContent}>
              <div className={styles.planGrid}>
                {plans.map((plan) => <PlanCard key={plan.name} plan={plan} />)}

                <article className={`${styles.planCard} ${styles.teamPlan}`}>
                  <div className={styles.planSummary}>
                    <h3>{msg("subscriptionPlansDialog.teamPlan")}</h3>
                    <p className={styles.description}>{msg("subscriptionPlansDialog.aBusinessPlanWithCustomPricingFor")}</p>
                    <button className={styles.managerButton} type="button">{msg("subscriptionPlansDialog.contactSales")}</button>
                  </div>
                  <ul className={styles.teamFeatureList}>
                    {teamFeatures.map(([title, description, icon]) => (
                      <li key={title}>
                        <AssetIcon className={styles.featureIcon} iconName={icon} source={assetPaths.icons.plans[icon]} />
                        <span><strong>{title}</strong><small>{description}</small></span>
                      </li>
                    ))}
                  </ul>
                </article>
              </div>

              <footer className={styles.dialogFooter}>
                <p>{msg("subscriptionPlansDialog.byPurchasingASubscriptionYouAgreeTo")}</p>
                <button type="button">{msg("subscriptionPlansDialog.learnMoreAboutPlans")}</button>
              </footer>
            </div>
          </ScrollArea>
        </section>}
      </ModalBackdrop>
      {promoOpen ? <PromoCodeDialog onClose={() => setPromoOpen(false)} returnFocusRef={promoButtonRef} /> : null}
    </>
  );
}
