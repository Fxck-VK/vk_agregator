import type { ConversationHistoryData } from "@/features/conversations/conversation-history-data";
import type { VideoJobList, VideoJobResult } from "@/features/files/VideoFiles/video-data";
import { createMusicResultCache, type MusicResultCache } from "@/features/music/music-result-cache";
import type { ImageJob, ImageJobList, ImageJobResult } from "@/lib/web-api/contracts";

export const maxCachedConversationHistoryPages = 8;

export type ReadyConversationHistory = Extract<ConversationHistoryData, { kind: "ready" }>;

export type ImageFileRetryReplacement = {
  originalJobID: string;
  job: ImageJob;
};

export type WorkspaceDataCache = {
  musicResults: MusicResultCache;
  getImageResults: () => Record<string, ImageJobResult>;
  setImageResult: (result: ImageJobResult) => void;
  getConversationHistory: (conversationId: string) => ReadyConversationHistory | undefined;
  setConversationHistory: (history: ConversationHistoryData) => void;
  deleteConversationHistory: (conversationId: string) => void;
  getImageFilesFirstPage: () => ImageJobList | undefined;
  setImageFilesFirstPage: (page: ImageJobList) => void;
  getVideoFilesFirstPage: () => VideoJobList | undefined;
  setVideoFilesFirstPage: (page: VideoJobList) => void;
  getVideoResults: () => Record<string, VideoJobResult>;
  setVideoResult: (result: VideoJobResult) => void;
  getImageFileRetryReplacements: () => ImageFileRetryReplacement[];
  setImageFileRetryReplacement: (replacement: ImageFileRetryReplacement) => void;
};

export function createWorkspaceDataCache(): WorkspaceDataCache {
  const conversationHistories = new Map<string, ReadyConversationHistory>();
  const imageFileRetryReplacements = new Map<string, ImageJob>();
  let imageFilesFirstPage: ImageJobList | undefined;
  const imageResults = new Map<string, ImageJobResult>();
  let videoFilesFirstPage: VideoJobList | undefined;
  const videoResults = new Map<string, VideoJobResult>();

  return {
    musicResults: createMusicResultCache(),
    getImageResults: () => Object.fromEntries(imageResults),
    setImageResult(result) {
      imageResults.delete(result.job_id);
      imageResults.set(result.job_id, result);
      if (imageResults.size > 60) imageResults.delete(imageResults.keys().next().value!);
    },
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
    getVideoFilesFirstPage() {
      return videoFilesFirstPage;
    },
    setVideoFilesFirstPage(page) {
      videoFilesFirstPage = page;
    },
    getVideoResults: () => Object.fromEntries(videoResults),
    setVideoResult(result) {
      videoResults.delete(result.job_id);
      videoResults.set(result.job_id, result);
      if (videoResults.size > 60) videoResults.delete(videoResults.keys().next().value!);
    },
    getImageFileRetryReplacements() {
      return [...imageFileRetryReplacements].map(([originalJobID, job]) => ({ originalJobID, job }));
    },
    setImageFileRetryReplacement({ originalJobID, job }) {
      imageFileRetryReplacements.set(originalJobID, job);
    },
  };
}
