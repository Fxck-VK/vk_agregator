import { clearDeckDrafts } from "@/features/presentations/deck-run";
import { clearPresentationChatAttempts } from "@/features/presentations/presentation-chat-api";
import { clearPendingConversationPrompts } from "@/features/conversations/pending-conversation-prompt";
import { clearPendingConversationBootstraps } from "@/features/conversations/pending-conversation-bootstrap";
import { clearPendingConversationTitleSyncs } from "@/features/conversations/pending-conversation-title-sync";
import { clearConversationCache } from "@/features/conversations/conversation-list-cache";
import { endBrowserSession } from "@/lib/web-api/browser-session-state";
import { clearModelCatalogCache } from "@/features/models/model-catalog-cache";

export function clearPrivateBrowserState() {
  endBrowserSession();
  clearConversationCache();
  clearModelCatalogCache();
  clearDeckDrafts();
  clearPresentationChatAttempts();
  clearPendingConversationPrompts();
  clearPendingConversationBootstraps();
  clearPendingConversationTitleSyncs();
}
