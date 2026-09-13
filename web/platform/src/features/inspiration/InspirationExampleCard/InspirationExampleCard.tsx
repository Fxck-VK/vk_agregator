"use client";

import { useRef, useState } from "react";

import { ModelIcon } from "@/features/models/ModelIcon/ModelIcon";
import { ru } from "@/i18n/ru";

import { InspirationExampleMedia } from "../InspirationExampleMedia/InspirationExampleMedia";
import type { InspirationExample } from "../inspiration-examples";
import { InspirationExampleDialog } from "./InspirationExampleDialogTemplate";
import styles from "./InspirationExampleCard.module.css";

type InspirationExampleCardProps = {
  example: InspirationExample;
  onOpen?: (trigger: HTMLButtonElement) => void;
  priority?: boolean;
};

export function InspirationExampleCard({
  example,
  onOpen,
  priority = false,
}: InspirationExampleCardProps) {
  const [isOpen, setIsOpen] = useState(false);
  const cardRef = useRef<HTMLButtonElement>(null);
  const cardVideoRef = useRef<HTMLVideoElement>(null);
  const closeDialog = () => {
    setIsOpen(false);
    window.requestAnimationFrame(() => cardRef.current?.focus());
  };

  const playCardVideo = () => {
    const video = cardVideoRef.current;
    if (!video) return;

    video.currentTime = 0;
    void video.play().catch(() => undefined);
  };

  const resetCardVideo = () => {
    const video = cardVideoRef.current;
    if (!video) return;

    video.pause();
    video.currentTime = 0;
  };

  return (
    <>
      <button
        aria-label={example.openLabel}
        className={styles.card}
        onClick={(event) => {
          if (onOpen) {
            onOpen(event.currentTarget);
          } else {
            setIsOpen(true);
          }
        }}
        onMouseEnter={playCardVideo}
        onMouseLeave={resetCardVideo}
        ref={cardRef}
        type="button"
      >
        <InspirationExampleMedia
          className={styles.cardMedia}
          example={example}
          priority={priority}
          videoProps={{ loop: true, muted: true, playsInline: true, preload: "auto" }}
          videoRef={cardVideoRef}
        />
        <span className={styles.cardShade} />
        <span className={styles.cardAction}>{ru.inspiration.details}</span>
        <span className={styles.cardMeta}>
          <ModelIcon className={styles.modelMark} />
          <span>{example.modelName}</span>
        </span>
      </button>

      {isOpen && !onOpen ? (
        <InspirationExampleDialog
          examples={[example]}
          onClose={closeDialog}
          onSelect={() => undefined}
          selectedIndex={0}
        />
      ) : null}
    </>
  );
}
