import { z } from "zod";

export const rewardPolicySchema = z.object({
  version: z.literal(1),
  stars: z.number().int().positive().max(Number.MAX_SAFE_INTEGER),
  delay_hours: z.number().int().positive().max(Number.MAX_SAFE_INTEGER),
  expires: z.literal(false),
});

export type ReferralRewardPolicy = z.infer<typeof rewardPolicySchema>;

// Explanatory preview only; live rewards always require a validated API policy.
export const previewRewardPolicy: ReferralRewardPolicy = { version: 1, stars: 50, delay_hours: 24, expires: false };

export function formatRewardPolicy(template: string, policy: ReferralRewardPolicy, locale: string) {
  return template.replaceAll("{stars}", policy.stars.toLocaleString(locale))
    .replaceAll("{hours}", policy.delay_hours.toLocaleString(locale));
}
