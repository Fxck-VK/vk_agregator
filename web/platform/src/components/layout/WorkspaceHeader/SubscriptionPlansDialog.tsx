"use client";

import { useDictionary, useLocale, useMessages } from "@/i18n/LocaleProvider";
import Link from "@/i18n/Link";
import { useRouter } from "@/i18n/navigation";

import { useEffect, useRef, useState } from "react";

import { assetPaths } from "@/assets/asset-paths";
import { AssetIcon } from "@/components/icons/AssetIcon";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";

import { subscriptionPlanDialogValues, subscriptionPlanDiscountPercent, subscriptionPlanPreview, type SubscriptionPreviewId } from "@/features/pricing/subscription-preview";
import { formatPaymentPrice } from "@/features/payments/payments";
import { useSubscriptionCheckout } from "@/features/subscriptions/use-subscription-checkout";
import { useOptionalWorkspaceAccount } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { PaymentStatus } from "@/features/payments/PaymentStatus";
import { subscriptionCopy } from "@/features/subscriptions/copy";
import { PromoCodeDialog } from "./PromoCodeDialog";
import styles from "./SubscriptionPlansDialog.module.css";

type SubscriptionPlansDialogProps = {
  onClose: () => void;
};

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

function PlanCard({ plan, onActivate, disabled, busy }: { plan: Plan; onActivate: (id: SubscriptionPreviewId) => void; disabled: boolean; busy: boolean }) {
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

export function SubscriptionPlansDialog(props: SubscriptionPlansDialogProps) {
  const account = useOptionalWorkspaceAccount();
  return <SubscriptionPlansContent key={account?.profile.account_id ?? "guest"} {...props}/>;
}

function SubscriptionPlansContent({ onClose }: SubscriptionPlansDialogProps) {
  const msg = useMessages();
  const promoText = useDictionary().promotions;
  const router = useRouter();
  const checkout = useSubscriptionCheckout();
  const {account,state} = checkout;
  const locale = useLocale(), t = subscriptionCopy[locale];
  const trial = subscriptionPlanPreview("lite"), followingTrial = subscriptionPlanPreview(trial.next_plan_code);
  const liteVisible = !account || !state.pending && !state.failed && state.data?.trial_used === false;
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
  const feedbackRef = useRef<HTMLDivElement>(null);
  const [promoOpen, setPromoOpen] = useState(false);

  useEffect(() => {
    if (checkout.error || checkout.paymentID) feedbackRef.current?.focus();
  }, [checkout.error, checkout.paymentID]);

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
              <button aria-haspopup="dialog" aria-expanded={promoOpen} className={styles.promoButton} disabled={checkout.busy || checkout.paymentID !== null} onClick={() => setPromoOpen(true)} ref={promoButtonRef} type="button">{checkout.promotion.quote ? promoText.applied(checkout.promotion.quote.code, checkout.promotion.quote.percent) : msg("subscriptionPlansDialog.activatePromoCode")}</button>
            </header>

            <div className={styles.plansContent}>
              <div className={styles.planGrid}>
                {plans.filter(plan => plan.id !== "lite" || liteVisible).map(plan => <PlanCard
                  key={plan.id} plan={{ ...plan, price: checkout.displayAmount(plan.id) === subscriptionPlanPreview(plan.id).amount ? plan.price : formatPaymentPrice(checkout.displayAmount(plan.id), locale), chargedAmount: checkout.displayAmount(plan.id), previousPrice: plan.previousPrice ?? (checkout.displayAmount(plan.id) !== subscriptionPlanPreview(plan.id).amount ? formatPaymentPrice(subscriptionPlanPreview(plan.id).amount, locale) : undefined) }} busy={checkout.busyPlanID === plan.id}
                  disabled={Boolean(account) && (!checkout.available(plan.id) || checkout.busy || checkout.paymentID !== null)}
                  onActivate={id => { if (!account) router.push("/login"); else void checkout.purchase(id); }}
                />)}

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
                {state.data?.recurring_available ? <label className={styles.renewalChoice}>
                  <input type="checkbox" checked={checkout.consent} disabled={checkout.busy || checkout.paymentID !== null}
                    onChange={event => checkout.setConsent(event.target.checked)}/>
                  <span>{t.autoRenewLabel}</span>
                </label> : null}
                {checkout.autoRenew ? <details className={styles.renewalDetails}>
                  <summary>{t.renewalDetails}</summary>
                  {liteVisible ? <p>{trial.name} → {followingTrial.name}: {formatPaymentPrice(followingTrial.amount,locale)} ₽ / {followingTrial.period_days} {t.days}, {t.after} {trial.period_days} {t.days}.</p> : null}
                  <p>{t.consent}</p><p>{t.expiry}</p>
                </details> : null}
                <p><Link href="/legal/user-agreement" target="_blank" rel="noopener noreferrer">{t.agreementLink}</Link>
                  {" · "}<Link href="/legal/paid-access" target="_blank" rel="noopener noreferrer">{t.paymentTerms}</Link></p>
                {account && state.pending ? <StateNotice inline kind="loading">{t.loading}</StateNotice> : null}
                {account && state.data && !state.failed && !state.pending && !state.data.checkout_available ? <StateNotice inline>{t.unavailable}</StateNotice> : null}
                {state.failed ? <StateNotice inline kind="error" action={{label:t.retry,onClick:state.retry}}>{t.error}</StateNotice> : null}
                <div ref={feedbackRef} tabIndex={-1}>
                  {checkout.error ? <StateNotice inline kind="error">{checkout.error}</StateNotice> : null}
                  {checkout.paymentID ? <PaymentStatus paymentID={checkout.paymentID} onDone={checkout.resetPayment}/> : null}
                </div>
                <button onClick={() => { onClose(); router.push("/pricing"); }} type="button">{msg("subscriptionPlansDialog.learnMoreAboutPlans")}</button>
              </footer>
            </div>
          </ScrollArea>
        </section>}
      </ModalBackdrop>
      {promoOpen ? <PromoCodeDialog onClose={() => setPromoOpen(false)} returnFocusRef={promoButtonRef} /> : null}
    </>
  );
}
