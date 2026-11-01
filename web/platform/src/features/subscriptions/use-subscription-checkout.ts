"use client";

import {useEffect,useRef,useState} from "react";
import {useOptionalWorkspaceAccount} from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import {checkoutNavigation} from "@/features/payments/checkout-navigation";
import {clearPaymentAttempt,paymentAttemptKey,PaymentRequestError,savePendingPayment} from "@/features/payments/payments";
import {subscriptionPlanPreview,type SubscriptionPreviewId} from "@/features/pricing/subscription-preview";
import {useReadResource} from "@/lib/use-read-resource";
import {useLocale} from "@/i18n/LocaleProvider";
import {browserSessionSnapshot} from "@/lib/web-api/browser-session-state";
import economy from "@/config/economy.generated.json";
import {createSubscriptionCheckout,loadSubscription,type SubscriptionCheckoutRequest} from "./subscriptions";
import {subscriptionCopy} from "./copy";
import {useSubscriptionPromotion} from "./SubscriptionPromotion";

/** Mount within the account-keyed purchase surface. All entrypoints share the
 * same frozen request, price verification and navigation rules. */
export function useSubscriptionCheckout(preview=false){
 const locale=useLocale(),t=subscriptionCopy[locale],account=useOptionalWorkspaceAccount();
 const promotion=useSubscriptionPromotion();
 const state=useReadResource(loadSubscription,null,Boolean(account)&&!preview,account?.profile);
 const [consent,setConsentState]=useState(false),[busyPlanID,setBusyPlanID]=useState<SubscriptionPreviewId|null>(null),[error,setError]=useState("");
 const [paymentID,setPaymentID]=useState<string|null>(null);
 const attempt=useRef<{request:SubscriptionCheckoutRequest;key:string;amount:number}|null>(null),submitting=useRef(false),alive=useRef(true);
 useEffect(()=>{alive.current=true;return()=>{alive.current=false;};},[]);
 const autoRenew=consent&&state.data?.recurring_available===true;
 function available(planID:SubscriptionPreviewId){const plan=subscriptionPlanPreview(planID);return !preview&&!promotion.pending&&Boolean(account)&&!state.failed&&!state.pending&&state.data?.checkout_available===true&&(!plan.trial||state.data?.trial_used===false);}
 function promotionFor(planID:SubscriptionPreviewId){return promotion.quote?.plans.find(plan=>plan.plan_code===planID);}
 function displayAmount(planID:SubscriptionPreviewId){return promotionFor(planID)?.amount??subscriptionPlanPreview(planID).amount;}
 function setConsent(value:boolean){if(submitting.current)return;setConsentState(value);attempt.current=null;clearPaymentAttempt();setError("");}
 async function purchase(planID:SubscriptionPreviewId){
  if(!available(planID)||submitting.current)return;
  const session=browserSessionSnapshot();
  if(session.accountId&&session.accountId!==account?.profile.account_id)return;
  const current=()=>alive.current&&browserSessionSnapshot().revision===session.revision;
  const plan=subscriptionPlanPreview(planID);
  const discount=promotionFor(planID),quote=discount?promotion.quote:null;
  submitting.current=true;setBusyPlanID(planID);setError("");
  if(attempt.current?.request.plan_code!==planID||attempt.current?.request.promo_code!==quote?.code||attempt.current?.request.promo_revision!==quote?.revision)attempt.current=null;
  attempt.current??={request:{plan_code:plan.id,auto_renew:autoRenew,...(autoRenew?{retry_policy_version:economy.renewal_retries.version}:{}),...(quote?{promo_code:quote.code,promo_revision:quote.revision}:{})},key:paymentAttemptKey(`subscription:${plan.id}:${autoRenew}${autoRenew?`:retry:${economy.renewal_retries.version}`:""}${quote?`:promo:${quote.code}:${quote.revision}`:""}`),amount:discount?.amount??plan.amount};
  const submitted=attempt.current;
  try{
   const payment=await createSubscriptionCheckout(submitted.request,submitted.key);
   if(!current())return;
   if(payment.amount!==submitted.amount||payment.credits!==plan.stars){setError(t.conflict);return;}
   savePendingPayment(payment.id);clearPaymentAttempt();
   if(payment.status==="waiting_for_user"&&payment.confirmation_url)checkoutNavigation.open(payment.confirmation_url);
   else setPaymentID(payment.id);
  }catch(cause){if(current())setError(cause instanceof PaymentRequestError&&cause.status===409?t.conflict:t.purchaseError);}
  finally{submitting.current=false;if(alive.current)setBusyPlanID(null);}
 }
 function resetPayment(){setPaymentID(null);attempt.current=null;state.retry();}
 return{account,state,consent,setConsent,autoRenew,busy:busyPlanID!==null,busyPlanID,error,paymentID,purchase,available,resetPayment,promotion,displayAmount};
}
