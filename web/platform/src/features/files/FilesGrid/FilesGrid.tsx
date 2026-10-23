"use client";

import type { ImageJob, ImageJobResult } from "@/lib/web-api/contracts";
import { MasonryGrid } from "@/components/ui/MasonryGrid/MasonryGrid";

import { FileCard, type FileResultState } from "../FileCard/FileCard";
import { useArtifactDeletion } from "../artifact-deletion";

type FilesGridProps = {
  jobs: ImageJob[];
  onOpenPreview: (
    job: ImageJob,
    artifact: ImageJobResult["artifacts"][number],
    trigger: HTMLButtonElement,
  ) => void;
  onRetryJob: (job: ImageJob) => void;
  onRequestResult: (job: ImageJob) => void;
  retryingJobIDs: ReadonlySet<string>;
  resultsByJobID: Record<string, ImageJobResult>;
  resultStatesByJobID: Record<string, FileResultState>;
};

export function FilesGrid({ jobs, onOpenPreview, onRequestResult, onRetryJob, resultsByJobID, resultStatesByJobID, retryingJobIDs }: Readonly<FilesGridProps>) {
  const deletion = useArtifactDeletion();
  const visibleJobs = jobs.filter(job => !resultsByJobID[job.id] || resultsByJobID[job.id].artifacts.some(artifact => !deletion.isDeleted(artifact.id, artifact.deleted)));
  return (
    <MasonryGrid>
      {visibleJobs.map((job) => (
        <li key={job.id}>
          <FileCard
            isRetrying={retryingJobIDs.has(job.id)}
            job={job}
            onOpenPreview={onOpenPreview}
            onRequestResult={onRequestResult}
            onRetryJob={onRetryJob}
            result={resultsByJobID[job.id] ?? null}
            resultState={resultStatesByJobID[job.id] ?? "idle"}
          />
        </li>
      ))}
    </MasonryGrid>
  );
}
