"use client";

import type { ImageJob, ImageJobResult } from "@/lib/web-api/contracts";
import { MasonryGrid } from "@/components/ui/MasonryGrid/MasonryGrid";

import { FileCard, type FileResultState } from "../FileCard/FileCard";

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
  return (
    <MasonryGrid>
      {jobs.map((job) => (
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
