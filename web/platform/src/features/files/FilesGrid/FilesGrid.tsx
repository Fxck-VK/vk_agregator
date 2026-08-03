"use client";

import type { ImageJob, ImageJobResult } from "@/lib/web-api/contracts";

import { FileCard, type FileResultState } from "../FileCard/FileCard";

import styles from "./FilesGrid.module.css";

type FilesGridProps = {
  jobs: ImageJob[];
  onRetryJob: (job: ImageJob) => void;
  onRequestResult: (job: ImageJob) => void;
  retryingJobIDs: ReadonlySet<string>;
  resultsByJobID: Record<string, ImageJobResult>;
  resultStatesByJobID: Record<string, FileResultState>;
};

export function FilesGrid({ jobs, onRequestResult, onRetryJob, resultsByJobID, resultStatesByJobID, retryingJobIDs }: Readonly<FilesGridProps>) {
  return (
    <ol className={styles.grid}>
      {jobs.map((job) => (
        <li key={job.id}>
          <FileCard
            isRetrying={retryingJobIDs.has(job.id)}
            job={job}
            onRequestResult={onRequestResult}
            onRetryJob={onRetryJob}
            result={resultsByJobID[job.id] ?? null}
            resultState={resultStatesByJobID[job.id] ?? "idle"}
          />
        </li>
      ))}
    </ol>
  );
}
