"use client";

import { useDictionary, useLocale } from "@/i18n/LocaleProvider";


import { useRef, useState } from "react";

import { WorkspacePageFrame } from "@/components/layout/WorkspacePageFrame/WorkspacePageFrame";
import { MasonryGrid } from "@/components/ui/MasonryGrid/MasonryGrid";

import { InspirationExampleCard } from "../InspirationExampleCard/InspirationExampleCard";
import { InspirationExampleDialog } from "../InspirationExampleCard/InspirationExampleDialogTemplate";
import { getInspirationExamples } from "../inspiration-examples";
import styles from "./InspirationGallery.module.css";

export function InspirationGallery() {
  const t = useDictionary();
  const inspirationExamples = getInspirationExamples(useLocale());
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
          <h1 id="inspiration-title">{t.inspiration.title}</h1>
          <p>{t.inspiration.description}</p>
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
