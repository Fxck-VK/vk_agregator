"use client";

import { useState, useSyncExternalStore } from "react";
import { createWorkspaceDataCache, useOptionalWorkspaceDataCache } from "@/features/workspace/WorkspaceDataCache/WorkspaceDataCache";

export function artifactIDFromPath(path: string | null | undefined) {
  return path?.match(/^\/web\/v1\/(?:image|video|music|speech)-artifacts\/([a-f0-9-]{36})$/i)?.[1];
}

export function useArtifactDeletion() {
  const workspace = useOptionalWorkspaceDataCache();
  const [fallback] = useState(() => workspace ?? createWorkspaceDataCache());
  const cache = workspace ?? fallback;
  const deletedIDs = useSyncExternalStore(cache.subscribeArtifactDeletion, cache.getDeletedArtifactIDs, cache.getDeletedArtifactIDs);
  return { deletedIDs, markDeleted: cache.markArtifactDeleted, isDeleted: (id: string, deleted = false) => deleted || deletedIDs.includes(id) };
}
