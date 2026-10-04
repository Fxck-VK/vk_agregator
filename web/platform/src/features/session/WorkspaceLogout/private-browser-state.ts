import { clearPendingConversationPrompts } from "@/features/conversations/pending-conversation-prompt";
import { clearPendingConversationBootstraps } from "@/features/conversations/pending-conversation-bootstrap";
import { clearPendingConversationTitleSyncs } from "@/features/conversations/pending-conversation-title-sync";
import { clearConversationCache } from "@/features/conversations/conversation-list-cache";
import { endBrowserSession } from "@/lib/web-api/browser-session-state";

export function clearPrivateBrowserState() {
  endBrowserSession();
  clearConversationCache();
  clearPendingConversationPrompts();
  clearPendingConversationBootstraps();
  clearPendingConversationTitleSyncs();
}
