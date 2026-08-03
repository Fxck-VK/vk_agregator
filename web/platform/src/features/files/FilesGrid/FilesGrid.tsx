"use client";

import type { ImageJob, ImageJobResult } from "@/lib/web-api/contracts";

import { FileCard, type FileResultState } from "../FileCard/FileCard";

import styles from "./FilesGrid.module.css";

type FilesGridProps = {
  jobs: ImageJob[];
  onRequestResult: (job: ImageJob) => void;
  resultsByJobID: Record<string, ImageJobResult>;
  resultStatesByJobID: Record<string, FileResultState>;
};

export function FilesGrid({ jobs, onRequestResult, resultsByJobID, resultStatesByJobID }: Readonly<FilesGridProps>) {
  return (
    <ol className={styles.grid}>
      {jobs.map((job) => (
        <li key={job.id}>
          <FileCard
            job={job}
            onRequestResult={onRequestResult}
            result={resultsByJobID[job.id] ?? null}
            resultState={resultStatesByJobID[job.id] ?? "idle"}
          />
        </li>
      ))}
    </ol>
  );
}
