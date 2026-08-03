import type { ConversationHistoryData } from "@/features/conversations/conversation-history-data";
import type { ImageJob, ImageJobList } from "@/lib/web-api/contracts";

export const maxCachedConversationHistoryPages = 8;

export type ReadyConversationHistory = Extract<ConversationHistoryData, { kind: "ready" }>;

export type ImageFileRetryReplacement = {
  originalJobID: string;
  job: ImageJob;
};

export type WorkspaceDataCache = {
  getConversationHistory: (conversationId: string) => ReadyConversationHistory | undefined;
  setConversationHistory: (history: ConversationHistoryData) => void;
  deleteConversationHistory: (conversationId: string) => void;
  getImageFilesFirstPage: () => ImageJobList | undefined;
  setImageFilesFirstPage: (page: ImageJobList) => void;
  getImageFileRetryReplacements: () => ImageFileRetryReplacement[];
  setImageFileRetryReplacement: (replacement: ImageFileRetryReplacement) => void;
};

export function createWorkspaceDataCache(): WorkspaceDataCache {
  const conversationHistories = new Map<string, ReadyConversationHistory>();
  const imageFileRetryReplacements = new Map<string, ImageJob>();
  let imageFilesFirstPage: ImageJobList | undefined;

  return {
    getConversationHistory(conversationId) {
      const history = conversationHistories.get(conversationId);
      if (history === undefined) {
        return undefined;
      }

      conversationHistories.delete(conversationId);
      conversationHistories.set(conversationId, history);
      return history;
    },
    setConversationHistory(history) {
      if (history.kind !== "ready") {
        return;
      }

      conversationHistories.delete(history.conversationId);
      conversationHistories.set(history.conversationId, history);
      if (conversationHistories.size > maxCachedConversationHistoryPages) {
        const oldestConversationId = conversationHistories.keys().next().value;
        if (oldestConversationId !== undefined) {
          conversationHistories.delete(oldestConversationId);
        }
      }
    },
    deleteConversationHistory(conversationId) {
      conversationHistories.delete(conversationId);
    },
    getImageFilesFirstPage() {
      return imageFilesFirstPage;
    },
    setImageFilesFirstPage(page) {
      imageFilesFirstPage = page;
    },
    getImageFileRetryReplacements() {
      return [...imageFileRetryReplacements].map(([originalJobID, job]) => ({ originalJobID, job }));
    },
    setImageFileRetryReplacement({ originalJobID, job }) {
      imageFileRetryReplacements.set(originalJobID, job);
    },
  };
}
