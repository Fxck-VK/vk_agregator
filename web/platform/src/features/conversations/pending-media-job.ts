import { z } from "zod";
const schema = z.object({jobID:z.string().uuid(),baselineSeq:z.number().int().nonnegative(),startedAt:z.number().int().positive()}).strict();
const key = (id:string) => `neirohub:conversation-pending-media:${id}`;
export function readPendingMediaJob(conversationID:string) {
 try {
  const raw = window.sessionStorage.getItem(key(conversationID));
  if (!raw) return null;
  const result = schema.safeParse(JSON.parse(raw));
  return result.success && Date.now()-result.data.startedAt < 60*60*1000 ? result.data : null;
 } catch { return null; }
}
export function savePendingMediaJob(conversationID:string,jobID:string,baselineSeq:number) {
 try { window.sessionStorage.setItem(key(conversationID),JSON.stringify({jobID,baselineSeq,startedAt:Date.now()})); } catch { /* optional storage */ }
}
export function clearPendingMediaJob(conversationID:string) {
 try { window.sessionStorage.removeItem(key(conversationID)); } catch { /* optional storage */ }
}
