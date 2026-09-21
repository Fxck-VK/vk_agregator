"use client";

import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { MediaImage } from "@/components/media/MediaImage/MediaImage";
import { useDictionary } from "@/i18n/LocaleProvider";


import { useState } from "react";

import { Button } from "@/components/ui/Button/Button";
import type { ImageJobResult } from "@/lib/web-api/contracts";

import styles from "./ImageGenerationResult.module.css";

type ImageGenerationResultProps = {
  onCreateAnother: (prompt: string) => void;
  prompt: string;
  result: ImageJobResult;
};

export function ImageGenerationResult({ onCreateAnother, prompt, result }: Readonly<ImageGenerationResultProps>) {
  const t = useDictionary();
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
        <h3 id="image-result-title">{t.imageGeneration.resultTitle}</h3>
        <p>{t.imageGeneration.resultReadyDescription}</p>
      </header>
      <div className={styles.artifacts}>
        {result.artifacts.map((artifact) => {
          const artifactPath = `/web/v1/image-artifacts/${artifact.id}`;
          return (
            <figure key={artifact.id}>
              <MediaImage
                alt={t.imageGeneration.resultImageAlt}
                height={artifact.height || undefined}
                src={artifactPath}
                width={artifact.width || undefined}
              />
              <figcaption>
                <a download href={artifactPath}>
                  {t.imageGeneration.downloadResult}
                </a>
              </figcaption>
            </figure>
          );
        })}
      </div>
      <div className={styles.actions}>
        <Button onClick={() => onCreateAnother(prompt)}>{t.imageGeneration.createAnother}</Button>
        <Button className={styles.secondaryAction} onClick={() => void copyPrompt()}>
          {t.imageGeneration.copyPrompt}
        </Button>
      </div>
      {copyState === "copied" ? <p aria-live="polite">{t.imageGeneration.promptCopied}</p> : null}
      {copyState === "failure" ? (
        <StateNotice inline kind="error">
          {t.imageGeneration.copyPromptFailure}
        </StateNotice>
      ) : null}
    </section>
  );
}
