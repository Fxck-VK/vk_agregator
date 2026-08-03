"use client";

/* eslint-disable @next/next/no-img-element */

import { useState } from "react";

import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";
import type { ImageJobResult } from "@/lib/web-api/contracts";

import styles from "./ImageGenerationResult.module.css";

type ImageGenerationResultProps = {
  onCreateAnother: (prompt: string) => void;
  prompt: string;
  result: ImageJobResult;
};

export function ImageGenerationResult({ onCreateAnother, prompt, result }: Readonly<ImageGenerationResultProps>) {
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failure">("idle");

  const copyPrompt = async () => {
    try {
      if (!navigator.clipboard) {
        throw new Error("Clipboard is unavailable.");
      }
      await navigator.clipboard.writeText(prompt);
      setCopyState("copied");
    } catch {
      setCopyState("failure");
    }
  };

  return (
    <section aria-labelledby="image-result-title" className={styles.result}>
      <header>
        <h3 id="image-result-title">{ru.imageGeneration.resultTitle}</h3>
        <p>{ru.imageGeneration.resultReadyDescription}</p>
      </header>
      <div className={styles.artifacts}>
        {result.artifacts.map((artifact) => {
          const artifactPath = `/web/v1/image-artifacts/${artifact.id}`;
          return (
            <figure key={artifact.id}>
              <img
                alt={ru.imageGeneration.resultImageAlt}
                height={artifact.height || undefined}
                src={artifactPath}
                width={artifact.width || undefined}
              />
              <figcaption>
                <a download href={artifactPath}>
                  {ru.imageGeneration.downloadResult}
                </a>
              </figcaption>
            </figure>
          );
        })}
      </div>
      <div className={styles.actions}>
        <Button onClick={() => onCreateAnother(prompt)}>{ru.imageGeneration.createAnother}</Button>
        <Button className={styles.secondaryAction} onClick={() => void copyPrompt()}>
          {ru.imageGeneration.copyPrompt}
        </Button>
      </div>
      {copyState === "copied" ? <p aria-live="polite">{ru.imageGeneration.promptCopied}</p> : null}
      {copyState === "failure" ? (
        <p className={styles.error} role="alert">
          {ru.imageGeneration.copyPromptFailure}
        </p>
      ) : null}
    </section>
  );
}
