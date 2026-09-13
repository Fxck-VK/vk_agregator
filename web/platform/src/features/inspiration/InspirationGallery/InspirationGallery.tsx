"use client";

import { useRef, useState } from "react";

import { WorkspacePageFrame } from "@/components/layout/WorkspacePageFrame/WorkspacePageFrame";
import { MasonryGrid } from "@/components/ui/MasonryGrid/MasonryGrid";
import { ru } from "@/i18n/ru";

import { InspirationExampleCard } from "../InspirationExampleCard/InspirationExampleCard";
import { InspirationExampleDialog } from "../InspirationExampleCard/InspirationExampleDialogTemplate";
import { inspirationExamples } from "../inspiration-examples";
import styles from "./InspirationGallery.module.css";

export function InspirationGallery() {
  const [selectedIndex, setSelectedIndex] = useState<number | null>(null);
  const openingCardRef = useRef<HTMLButtonElement | null>(null);

  const closeDialog = () => {
    setSelectedIndex(null);
    window.requestAnimationFrame(() => openingCardRef.current?.focus());
  };

  return (
    <WorkspacePageFrame>
      <section aria-labelledby="inspiration-title" className={styles.gallery}>
        <div className={styles.heading}>
          <h1 id="inspiration-title">{ru.inspiration.title}</h1>
          <p>{ru.inspiration.description}</p>
        </div>

        <MasonryGrid>
          {inspirationExamples.map((example, index) => (
            <li key={example.id}>
              <InspirationExampleCard
                example={example}
                onOpen={(trigger) => {
                  openingCardRef.current = trigger;
                  setSelectedIndex(index);
                }}
                priority={index === 0}
              />
            </li>
          ))}
        </MasonryGrid>
      </section>

      {selectedIndex !== null ? (
        <InspirationExampleDialog
          examples={inspirationExamples}
          onClose={closeDialog}
          onSelect={setSelectedIndex}
          selectedIndex={selectedIndex}
        />
      ) : null}
    </WorkspacePageFrame>
  );
}
