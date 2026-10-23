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
  subscribeArtifactDeletion: (listener: () => void) => () => void;
  getDeletedArtifactIDs: () => readonly string[];
  markArtifactDeleted: (id: string) => void;
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
  let deletedArtifactIDs: readonly string[] = [];
  const deletionListeners = new Set<() => void>();
  const withTombstones = <T extends { artifacts: Array<{ id: string; deleted?: boolean; size_bytes: number }> }>(result: T): T => ({
    ...result, artifacts: result.artifacts.map(artifact => deletedArtifactIDs.includes(artifact.id) ? { ...artifact, deleted: true, size_bytes: 0 } : artifact),
  });
  const conversationHistories = new Map<string, ReadyConversationHistory>();
  const imageFileRetryReplacements = new Map<string, ImageJob>();
  let imageFilesFirstPage: ImageJobList | undefined;
  const imageResults = new Map<string, ImageJobResult>();
  let videoFilesFirstPage: VideoJobList | undefined;
  const videoResults = new Map<string, VideoJobResult>();

  return {
    subscribeArtifactDeletion(listener) { deletionListeners.add(listener); return () => { deletionListeners.delete(listener); }; },
    getDeletedArtifactIDs: () => deletedArtifactIDs,
    markArtifactDeleted(id) {
      if (deletedArtifactIDs.includes(id)) return;
      deletedArtifactIDs = [...deletedArtifactIDs, id];
      for (const [jobID, result] of imageResults) imageResults.set(jobID, withTombstones(result));
      for (const [jobID, result] of videoResults) videoResults.set(jobID, withTombstones(result));
      conversationHistories.clear();
      deletionListeners.forEach(listener => listener());
    },
    musicResults: createMusicResultCache(),
    getImageResults: () => Object.fromEntries(imageResults),
    setImageResult(result) {
      imageResults.delete(result.job_id);
      imageResults.set(result.job_id, withTombstones(result));
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
      videoResults.set(result.job_id, withTombstones(result));
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
