import policy from "@/config/economy.generated.json";

export type SubscriptionPreviewId = "lite" | "start" | "pro" | "ultima" | "elite";
export type SubscriptionPreviewPlan = {
  id: SubscriptionPreviewId;
  name: string;
  amount: number;
  stars: number;
  period_days: number;
  period: "week" | "month";
  trial: boolean;
  next_plan_code: SubscriptionPreviewId;
};
/** Presentation adapter over the generated, versioned server economic policy. */
export const subscriptionPreviewPlans: readonly SubscriptionPreviewPlan[] = policy.plans.map(plan => ({
  ...plan,
  id: plan.code as SubscriptionPreviewId,
  next_plan_code: (plan.next_plan_code || plan.code) as SubscriptionPreviewId,
  period: plan.period_days === 7 ? "week" : "month",
}));
export function subscriptionPlanPreview(id: SubscriptionPreviewId): SubscriptionPreviewPlan {
  const plan = subscriptionPreviewPlans.find(plan => plan.id === id);
  if (!plan) throw new Error("Unknown subscription plan");
  return plan;
}
export function subscriptionPlanDialogValues(id: SubscriptionPreviewId) {
  const plan = subscriptionPlanPreview(id);
  return { id: plan.id, name: plan.name, price: String(plan.amount / 100), credits: String(plan.stars) };
}

/** Display whole percentages, rounded down so a badge never overstates savings. */
export function subscriptionPlanDiscountPercent(id: SubscriptionPreviewId, previousPrice?: string): number | null {
  const previousAmount = Number(previousPrice) * 100;
  const difference = previousAmount - subscriptionPlanPreview(id).amount;
  if (!Number.isSafeInteger(previousAmount) || previousAmount <= 0 || difference <= 0 || !Number.isSafeInteger(difference * 100)) return null;
  const percent = Math.floor(difference * 100 / previousAmount);
  return percent > 0 ? percent : null;
}
