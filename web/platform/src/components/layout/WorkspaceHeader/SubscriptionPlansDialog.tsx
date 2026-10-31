"use client";

import { useLocale, useMessages } from "@/i18n/LocaleProvider";
import Link from "@/i18n/Link";
import { RichMessage } from "@/i18n/RichMessage";
import { useRouter } from "@/i18n/navigation";

import { useEffect, useRef, useState, type RefObject } from "react";

import { assetPaths } from "@/assets/asset-paths";
import { AssetIcon } from "@/components/icons/AssetIcon";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";

import { subscriptionPlanDialogValues, subscriptionPlanDiscountPercent, type SubscriptionPreviewId } from "@/features/pricing/subscription-preview";
import { SubscriptionCheckout } from "@/features/subscriptions/SubscriptionCheckout";
import { useLiteOffer } from "@/features/subscriptions/use-lite-offer";
import { subscriptionCopy } from "@/features/subscriptions/copy";
import { PromoCodeDialog } from "./PromoCodeDialog";
import styles from "./SubscriptionPlansDialog.module.css";
import paymentStyles from "./TokenTopUpDialog.module.css";

type SubscriptionPlansDialogProps = {
  onClose: () => void;
};

type Plan = {
  id: SubscriptionPreviewId;
  name: string;
  price: string;
  period: string;
  previousPrice?: string;
  description: string;
  creditPeriod: string;
  credits: string;
  features: {
    icon: "imageGeneration" | "videoGeneration" | "aiModels" | "presentationGeneration";
    text: string;
  }[];
  popular?: boolean;
};

function PlanCard({ plan, onActivate }: { plan: Plan; onActivate: (id: SubscriptionPreviewId, trigger: HTMLButtonElement) => void }) {
  const msg = useMessages();
  const percent = subscriptionPlanDiscountPercent(plan.id, plan.previousPrice);
  const discount = percent === null ? undefined : `−${percent}%`;
  return (
    <article className={`${styles.planCard} ${plan.popular ? styles.popularPlan : ""}`}>
      {plan.popular ? <span className={styles.popularLabel}>{msg("subscriptionPlansDialog.popular")}</span> : null}
      {discount ? <span className={styles.discount}>{discount}</span> : null}

      <div className={styles.planSummary}>
        <h3>{plan.name}</h3>
        <p className={styles.price}>
          <strong>{plan.price}</strong> <span>{plan.period}</span>{" "}
          {plan.previousPrice ? <del>{plan.previousPrice}</del> : null}
        </p>
        <p className={styles.description}>{plan.description}</p>
        <button className={styles.subscribeButton} onClick={(event) => onActivate(plan.id, event.currentTarget)} type="button">
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

function SubscriptionCheckoutDialog({ planID, onClose, returnFocusRef, fallbackFocusRef }: {
  planID: SubscriptionPreviewId;
  onClose: () => void;
  returnFocusRef: RefObject<HTMLButtonElement | null>;
  fallbackFocusRef: RefObject<HTMLButtonElement | null>;
}) {
  const msg = useMessages();
  const locale = useLocale();
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    const trigger = returnFocusRef.current;
    const fallback = fallbackFocusRef.current;
    closeButtonRef.current?.focus();
    return () => { (trigger?.isConnected ? trigger : fallback)?.focus(); };
  }, [returnFocusRef, fallbackFocusRef]);

  return <ModalBackdrop onClose={onClose} testId="subscription-checkout-backdrop">
    {(requestClose) => <div aria-labelledby="subscription-checkout-title" aria-modal="true" className={paymentStyles.dialogLayer} role="dialog">
      <ModalCloseButton aria-label={msg("subscriptionPlansDialog.closePlans")} className={paymentStyles.closeButtonPlacement} onClick={requestClose} ref={closeButtonRef} />
      <section className={paymentStyles.dialog}>
        <header className={paymentStyles.header}><h2 id="subscription-checkout-title">{subscriptionCopy[locale].offer}</h2></header>
        <ScrollArea><SubscriptionCheckout planID={planID} /></ScrollArea>
      </section>
    </div>}
  </ModalBackdrop>;
}

export function SubscriptionPlansDialog({ onClose }: SubscriptionPlansDialogProps) {
  const msg = useMessages();
  const router = useRouter();
  const liteOffer = useLiteOffer();
  const plans: Plan[] = [
    {
      ...subscriptionPlanDialogValues("lite"),
      period: msg("subscriptionPlansDialog.week"),
      description: msg("subscriptionPlansDialog.trialPlan"),
      creditPeriod: msg("subscriptionPlansDialog.oneTime"),
      features: [
        { icon: "imageGeneration", text: msg("subscriptionPlansDialog.upTo13ImageGenerationsNanoBanana") },
        { icon: "videoGeneration", text: msg("subscriptionPlansDialog.upTo2VideoGenerationsVideoGenerator") },
        { icon: "aiModels", text: msg("subscriptionPlansDialog.accessToPopularAiModelsChatgptGemini") },
      ],
    },
    {
      ...subscriptionPlanDialogValues("start"),
      period: msg("subscriptionPlansDialog.month"),
      previousPrice: "632",
      description: msg("subscriptionPlansDialog.aGreatPlaceToStart"),
      creditPeriod: msg("subscriptionPlansDialog.monthly"),
      features: [
        { icon: "imageGeneration", text: msg("subscriptionPlansDialog.upTo41ImageGenerationsWithPopular") },
        { icon: "videoGeneration", text: msg("subscriptionPlansDialog.upTo15VideoGenerations") },
        { icon: "aiModels", text: msg("subscriptionPlansDialog.accessToChatgptGeminiClaudeSunoKimi") },
        { icon: "presentationGeneration", text: msg("subscriptionPlansDialog.4PresentationGenerations") },
      ],
    },
    {
      ...subscriptionPlanDialogValues("pro"),
      period: msg("subscriptionPlansDialog.month"),
      previousPrice: "1200",
      description: msg("subscriptionPlansDialog.aBalancedChoice"),
      creditPeriod: msg("subscriptionPlansDialog.monthly"),
      features: [
        { icon: "imageGeneration", text: msg("subscriptionPlansDialog.upTo75ImageGenerationsWithPopular") },
        { icon: "videoGeneration", text: msg("subscriptionPlansDialog.upTo15VideoGenerations") },
        { icon: "aiModels", text: msg("subscriptionPlansDialog.accessToAllCoreAiModels") },
        { icon: "presentationGeneration", text: msg("subscriptionPlansDialog.7PresentationGenerations") },
      ],
    },
    {
      ...subscriptionPlanDialogValues("ultima"),
      period: msg("subscriptionPlansDialog.month"),
      previousPrice: "2600",
      description: msg("subscriptionPlansDialog.morePossibilities"),
      creditPeriod: msg("subscriptionPlansDialog.monthly"),
      features: [
        { icon: "imageGeneration", text: msg("subscriptionPlansDialog.upTo160ImageGenerations") },
        { icon: "videoGeneration", text: msg("subscriptionPlansDialog.upTo32VideoGenerations") },
        { icon: "aiModels", text: msg("subscriptionPlansDialog.extendedAiModelAccess") },
        { icon: "presentationGeneration", text: msg("subscriptionPlansDialog.16PresentationGenerations") },
      ],
      popular: true,
    },
    {
      ...subscriptionPlanDialogValues("elite"),
      period: msg("subscriptionPlansDialog.month"),
      previousPrice: "7000",
      description: msg("subscriptionPlansDialog.maximumValue"),
      creditPeriod: msg("subscriptionPlansDialog.monthly"),
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
  const [checkoutPlan, setCheckoutPlan] = useState<SubscriptionPreviewId | null>(null);
  const checkoutButtonRef = useRef<HTMLButtonElement | null>(null);

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
      <ModalBackdrop closeOnBackdropClick={!promoOpen && checkoutPlan === null} closeOnEscape={!promoOpen && checkoutPlan === null} onClose={onClose} testId="subscription-plans-backdrop">
        {(requestClose) => <section
          aria-hidden={promoOpen || checkoutPlan !== null || undefined}
          aria-labelledby="subscription-plans-title"
          aria-modal={!promoOpen && checkoutPlan === null}
          className={styles.dialog}
          inert={promoOpen || checkoutPlan !== null}
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
                {plans.filter(plan => plan.id !== "lite" || liteOffer.visible).map((plan) => <PlanCard key={plan.name} plan={plan} onActivate={(id, trigger) => {
                  checkoutButtonRef.current = trigger;
                  setCheckoutPlan(id);
                }} />)}

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
                <p><RichMessage id="subscriptionPlansDialog.byPurchasingASubscriptionYouAgreeTo" values={{
                  agreement: <Link href="/legal/user-agreement" target="_blank" rel="noopener noreferrer">{msg("subscriptionPlansDialog.userAgreement")}</Link>,
                  recurring: <Link href="/legal/paid-access" target="_blank" rel="noopener noreferrer">{msg("subscriptionPlansDialog.recurringPayments")}</Link>,
                }} /></p>
                <button onClick={() => { onClose(); router.push("/pricing"); }} type="button">{msg("subscriptionPlansDialog.learnMoreAboutPlans")}</button>
              </footer>
            </div>
          </ScrollArea>
        </section>}
      </ModalBackdrop>
      {promoOpen ? <PromoCodeDialog onClose={() => setPromoOpen(false)} returnFocusRef={promoButtonRef} /> : null}
      {checkoutPlan ? <SubscriptionCheckoutDialog onClose={() => { setCheckoutPlan(null); liteOffer.refresh(); }} planID={checkoutPlan} returnFocusRef={checkoutButtonRef} fallbackFocusRef={closeButtonRef} /> : null}
    </>
  );
}
