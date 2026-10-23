import { z } from "zod";
export const legalSlugs = ["user-agreement", "privacy-policy", "paid-access", "payment-details", "marketing-consent"] as const;
export type LegalSlug = typeof legalSlugs[number];
export const legalMetadataSchema = z.object({
 published:z.boolean(),version:z.string().min(1).max(100),checksum:z.string().regex(/^[a-f0-9]{64}$/),
 marketing_version:z.string().min(1).max(100),marketing_checksum:z.string().regex(/^[a-f0-9]{64}$/),
});
export type LegalMetadata = z.infer<typeof legalMetadataSchema>;
export const legalStatusSchema = legalMetadataSchema.extend({accepted:z.boolean(),marketing_opt_in:z.boolean(),marketing_available:z.boolean(),recipient_version:z.string().regex(/^(?:[a-f0-9]{64})?$/).default("")});
export type LegalStatus = z.infer<typeof legalStatusSchema>;
export function sameLegalRevision(a:LegalMetadata,b:LegalMetadata){
 return a.published===b.published&&a.version===b.version&&a.checksum===b.checksum&&a.marketing_version===b.marketing_version&&a.marketing_checksum===b.marketing_checksum;
}
