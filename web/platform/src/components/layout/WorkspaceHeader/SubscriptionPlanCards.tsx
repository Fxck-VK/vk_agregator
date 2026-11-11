"use client";

import type { ReactNode } from "react";
import { useLocale, useMessages } from "@/i18n/LocaleProvider";
import { assetPaths } from "@/assets/asset-paths";
import { AssetIcon } from "@/components/icons/AssetIcon";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { subscriptionPlanDialogValues, subscriptionPlanDiscountPercent, subscriptionPlanPreview, type SubscriptionPreviewId } from "@/features/pricing/subscription-preview";
import { formatPaymentPrice } from "@/features/payments/payments";
import { subscriptionCopy } from "@/features/subscriptions/copy";
import styles from "./SubscriptionPlansDialog.module.css";

type Plan = {
  id: SubscriptionPreviewId;
  name: string;
  price: string;
  period: string;
  previousPrice?: string;
  chargedAmount?: number;
  description: string;
  creditPeriod: string;
  credits: string;
  features: {
    icon: "imageGeneration" | "videoGeneration" | "aiModels" | "presentationGeneration";
    text: string;
  }[];
  popular?: boolean;
};

function SubscriptionPlanCard({ plan, onActivate, disabled, busy }: { plan: Plan; onActivate: (id: SubscriptionPreviewId) => void; disabled: boolean; busy: boolean }) {
  const locale = useLocale();
  const msg = useMessages();
  const percent = subscriptionPlanDiscountPercent(plan.id, plan.previousPrice, plan.chargedAmount);
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
        <button className={styles.subscribeButton} disabled={disabled} onClick={() => onActivate(plan.id)} type="button">
          {busy ? subscriptionCopy[locale].pending : msg("subscriptionPlansDialog.activateSubscription")}</button>
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

type SubscriptionPlanCardsProps = {
  onActivate: (id: SubscriptionPreviewId) => void;
  liteVisible?: boolean;
  amountForPlan?: (id: SubscriptionPreviewId) => number;
  disabledPlan?: (id: SubscriptionPreviewId) => boolean;
  busyPlanID?: SubscriptionPreviewId | null;
  children?: ReactNode;
};

/** The same cards and economic projection for the landing and workspace. */
export function SubscriptionPlanCards({ onActivate, liteVisible = true, amountForPlan, disabledPlan, busyPlanID, children }: SubscriptionPlanCardsProps) {
  const msg = useMessages();
  const locale = useLocale();
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
  return <div className={styles.planGrid}>
    {plans.filter(plan => plan.id !== "lite" || liteVisible).map(plan => {
      const canonical = subscriptionPlanPreview(plan.id).amount;
      const amount = amountForPlan?.(plan.id) ?? canonical;
      return <SubscriptionPlanCard key={plan.id}
        plan={{ ...plan, price: amount === canonical ? plan.price : formatPaymentPrice(amount, locale), chargedAmount: amount,
          previousPrice: plan.previousPrice ?? (amount !== canonical ? formatPaymentPrice(canonical, locale) : undefined) }}
        busy={busyPlanID === plan.id} disabled={disabledPlan?.(plan.id) ?? false} onActivate={onActivate} />;
    })}
    {children}
  </div>;
}
