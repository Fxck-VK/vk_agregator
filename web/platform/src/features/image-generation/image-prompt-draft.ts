// Short-lived client-navigation handoff. Never written to a URL, browser storage,
// logs or an API. A reload or login navigation intentionally discards the draft.
let draft: { text: string; created: number } | null = null;
export function stageImagePrompt(text: string) { draft = { text: text.slice(0, 12000), created: Date.now() }; }
export function readImagePrompt() { return draft && Date.now() - draft.created < 5 * 60_000 ? draft.text : ""; }
export function clearImagePrompt(text: string) { if (draft?.text === text) draft = null; }
