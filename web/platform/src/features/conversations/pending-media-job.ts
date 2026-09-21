import { z } from "zod";
import type { GenerationOptions } from "@/features/models/generation-options-contract";
const imageSchema = z.object({ count: z.number().int().min(1).max(100), aspectRatio: z.string().regex(/^[1-9]\d*:[1-9]\d*$/) }).strict();
const schema = z.object({jobID:z.string().uuid(),baselineSeq:z.number().int().nonnegative(),startedAt:z.number().int().positive(),image:imageSchema.optional()}).strict();
// Persist only the image layout, never prompts or reference file identifiers.
export function pendingImagePreview(options?: GenerationOptions) {
 if (!options?.image_quality) return undefined;
 const parsed = imageSchema.safeParse({count:options.output_count ?? 1,aspectRatio:options.aspect_ratio ?? "1:1"});
 return parsed.success ? parsed.data : undefined;
}
const key = (id:string) => `neirohub:conversation-pending-media:${id}`;
export function readPendingMediaJob(conversationID:string) {
 try {
  const raw = window.sessionStorage.getItem(key(conversationID));
  if (!raw) return null;
  const result = schema.safeParse(JSON.parse(raw));
  return result.success && Date.now()-result.data.startedAt < 60*60*1000 ? result.data : null;
 } catch { return null; }
}
export function savePendingMediaJob(conversationID:string,jobID:string,baselineSeq:number,options?:GenerationOptions) {
 try { window.sessionStorage.setItem(key(conversationID),JSON.stringify({jobID,baselineSeq,startedAt:Date.now(),image:pendingImagePreview(options)})); } catch { /* optional storage */ }
}
export function clearPendingMediaJob(conversationID:string) {
 try { window.sessionStorage.removeItem(key(conversationID)); } catch { /* optional storage */ }
}
