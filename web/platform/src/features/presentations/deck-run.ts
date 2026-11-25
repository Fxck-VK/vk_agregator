import { z } from "zod";
import { presentationDeckCopy, presentationPromptFormat, presentationStyles } from "@/i18n/presentation-deck";
import type { Brief, DeckSlide } from "./deck-content";

const uuid = z.uuid();
const draftPrefix = "epifera:deck-draft:";
const draftSchema = z.strictObject({
  version: z.literal(1),
  slides: z.array(z.strictObject({
    title: z.string().min(1).max(160),
    points: z.array(z.string().min(1).max(300)).min(1).max(5),
  })).min(1).max(30),
});

export type DeckRun = { conversation: string; job?: string; pending?: true };

export function parseDeckRun(query: Record<string, string | string[] | undefined>): DeckRun | undefined {
  const conversation = uuid.safeParse(query.run);
  if (!conversation.success) return undefined;
  if (query.job === undefined) return { conversation: conversation.data, ...(query.pending === "1" ? { pending: true } : {}) };
  const job = uuid.safeParse(query.job);
  return job.success ? { conversation: conversation.data, job: job.data } : undefined;
}

export function replaceDeckRun(run?: DeckRun) {
  const url = new URL(window.location.href);
  url.searchParams.delete("run");
  url.searchParams.delete("job");
  url.searchParams.delete("pending");
  if (run) url.searchParams.set("run", run.conversation);
  if (run?.job) url.searchParams.set("job", run.job);
  if (run?.pending && !run.job) url.searchParams.set("pending", "1");
  window.history.replaceState(null, "", url);
}

export function readDeckBrief(prompt: string): Brief | undefined {
  const { briefPattern, audiencePrefix, notesPrefix, business } = presentationPromptFormat;
  const input = prompt.replace(/\r\n/g, "\n");
  const match = briefPattern.exec(input);
  if (!match) return undefined;
  const end = "\n\n" + presentationDeckCopy.prompt.format;
  const options = input.slice(match[0].length);
  const extra = options === presentationDeckCopy.prompt.format ? "" : options.endsWith(end) ? options.slice(0, -end.length) : options;
  const notesAt = extra.startsWith(notesPrefix) ? 0 : extra.indexOf("\n\n" + notesPrefix);
  const audience = extra.startsWith(audiencePrefix)
    ? extra.slice(audiencePrefix.length, notesAt < 0 ? undefined : notesAt).trim().replace(/\.$/, "")
    : "";
  const notes = notesAt < 0 ? "" : extra.slice(notesAt + (notesAt === 0 ? 0 : 2) + notesPrefix.length).trim();
  return {
    topic: match[1],
    count: match[2] === "10" ? 10 : 5,
    style: match[3] === business ? presentationStyles.business : presentationStyles.educational,
    audience,
    notes,
  };
}

const deckDraftKey = (account: string, job: string) => draftPrefix + account + ":" + job;

export function getDeckDraft(account: string, job: string): DeckSlide[] | undefined {
  try {
    const raw = sessionStorage.getItem(deckDraftKey(account, job));
    if (raw === null) return undefined;
    const value = draftSchema.safeParse(JSON.parse(raw));
    return value.success ? value.data.slides : undefined;
  } catch { return undefined; }
}

export function saveDeckDraft(account: string, job: string, slides: DeckSlide[]) {
  const value = draftSchema.safeParse({ version: 1, slides });
  if (!value.success) return;
  try { sessionStorage.setItem(deckDraftKey(account, job), JSON.stringify(value.data)); }
  catch { /* The current edit and PowerPoint export remain available. */ }
}

export function clearDeckDrafts() {
  try {
    for (let index = sessionStorage.length - 1; index >= 0; index--) {
      const key = sessionStorage.key(index);
      if (key?.startsWith(draftPrefix)) sessionStorage.removeItem(key);
    }
  } catch { /* Browser storage may be unavailable. */ }
}
