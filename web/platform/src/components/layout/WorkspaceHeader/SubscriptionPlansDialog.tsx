"use client";

import { useDictionary, useLocale, useMessages } from "@/i18n/LocaleProvider";
import Link from "@/i18n/Link";
import { useRouter } from "@/i18n/navigation";

import { useEffect, useRef, useState } from "react";

import { assetPaths } from "@/assets/asset-paths";
import { AssetIcon } from "@/components/icons/AssetIcon";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";

import { subscriptionPlanPreview } from "@/features/pricing/subscription-preview";
import { formatPaymentPrice } from "@/features/payments/payments";
import { useSubscriptionCheckout } from "@/features/subscriptions/use-subscription-checkout";
import { useReleaseView } from "@/features/release/ReleaseViewProvider";
import { useOptionalWorkspaceAccount } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { PaymentStatus } from "@/features/payments/PaymentStatus";
import { subscriptionCopy } from "@/features/subscriptions/copy";
import { SubscriptionPlanCards } from "./SubscriptionPlanCards";
import { PromoCodeDialog } from "./PromoCodeDialog";
import styles from "./SubscriptionPlansDialog.module.css";

type SubscriptionPlansDialogProps = {
  onClose: () => void;
};

export function SubscriptionPlansDialog(props: SubscriptionPlansDialogProps) {
  const account = useOptionalWorkspaceAccount();
  return <SubscriptionPlansContent key={account?.profile.account_id ?? "guest"} {...props}/>;
}

function SubscriptionPlansContent({ onClose }: SubscriptionPlansDialogProps) {
  const { enabled } = useReleaseView();
  const msg = useMessages();
  const promoText = useDictionary().promotions;
  const router = useRouter();
  const checkout = useSubscriptionCheckout();
  const {account,state} = checkout;
  const locale = useLocale(), t = subscriptionCopy[locale];
  const trial = subscriptionPlanPreview("lite"), followingTrial = subscriptionPlanPreview(trial.next_plan_code);
  const liteVisible = !account || !state.pending && !state.failed && state.data?.trial_used === false;
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
              <SubscriptionPlanCards liteVisible={liteVisible} amountForPlan={checkout.displayAmount}
                busyPlanID={checkout.busyPlanID}
                disabledPlan={id => Boolean(account) && (!checkout.available(id) || checkout.busy || checkout.paymentID !== null)}
                onActivate={id => { if (!account) router.push("/login"); else void checkout.purchase(id); }}>
                {enabled("teams") ? <article className={`${styles.planCard} ${styles.teamPlan}`}>
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
                </article> : null}
              </SubscriptionPlanCards>

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
