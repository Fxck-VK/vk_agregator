"use client";

import { useDictionary, useLocale } from "@/i18n/LocaleProvider";


import Link from "@/i18n/Link";
import { useState } from "react";

import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import { MasonryGrid } from "@/components/ui/MasonryGrid/MasonryGrid";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";
import selectableStyles from "@/components/ui/selectable-control.module.css";
import { InspirationExampleCard } from "@/features/inspiration/InspirationExampleCard/InspirationExampleCard";
import { selectInspirationExamples } from "@/features/inspiration/inspiration-examples";
import { useWorkspaceModelSelection } from "@/features/models/WorkspaceModelSelection/WorkspaceModelSelection";

import styles from "./ImageGenerationGuide.module.css";

type GuideTab = "guide" | "examples";

function MediaIcon() {
  return (
    <svg aria-hidden="true" className={styles.controlIcon} viewBox="0 0 24 24">
      <path d="M4 7.5A2.5 2.5 0 0 1 6.5 5h11A2.5 2.5 0 0 1 20 7.5v9a2.5 2.5 0 0 1-2.5 2.5h-11A2.5 2.5 0 0 1 4 16.5v-9Z" />
      <path d="m5 15 4.2-4.2 3.3 3.3 2.2-2.2L20 17" />
      <path d="M17.5 3v5M15 5.5h5" />
    </svg>
  );
}

function AspectIcon() {
  return (
    <svg aria-hidden="true" className={styles.controlIcon} viewBox="0 0 24 24">
      <rect height="16" rx="2.5" width="10" x="7" y="4" />
    </svg>
  );
}

function PromptPreview({ compact = false }: { compact?: boolean }) {
  const t = useDictionary();
  return (
    <InputSurface className={`${styles.promptPreview} ${compact ? styles.promptPreviewCompact : ""}`}>
      <p>{compact ? t.imageGeneration.guide.promptExampleShort : t.imageGeneration.guide.promptExampleLong}</p>
      {compact ? (
        <div className={styles.previewControls}>
          <span className={styles.previewControl}>
            <MediaIcon />
            {t.imageGeneration.guide.mediaControl}
          </span>
          <span className={styles.previewControl}>
            <AspectIcon />
            {t.imageGeneration.guide.aspectControl}
          </span>
        </div>
      ) : null}
    </InputSurface>
  );
}

function ResultPreview() {
  const t = useDictionary();
  return (
    <div aria-label={t.imageGeneration.guide.resultPreviewLabel} className={styles.resultPreview} role="img">
      <span className={styles.resultLayerBack} />
      <span className={styles.resultLayerFront}>
        <span className={styles.resultSun} />
        <span className={styles.resultMountain} />
      </span>
    </div>
  );
}

export function ImageGenerationGuide() {
  const t = useDictionary();
  const [activeTab, setActiveTab] = useState<GuideTab>("guide");
  const workspaceModelSelection = useWorkspaceModelSelection();
  const locale = useLocale();
  const guide = t.imageGeneration.guide;
  const examples = selectInspirationExamples(
    workspaceModelSelection?.selectedModelId,
    6,
    "image",
    { fillFromCollection: true },
    locale,
  );

  return (
    <section aria-label={guide.tabsLabel} className={styles.root}>
      <ScrollArea
        className={styles.tabsScroll}
        orientation="horizontal"
        viewportClassName={styles.tabs}
        viewportProps={{ "aria-label": guide.tabsLabel, role: "tablist" }}
      >
        <button
          aria-controls="image-generation-guide-panel"
          aria-selected={activeTab === "guide"}
          className={styles.tab}
          id="image-generation-guide-tab"
          onClick={() => setActiveTab("guide")}
          role="tab"
          type="button"
        >
          {guide.howToTab}
        </button>
        <button
          aria-controls="image-generation-examples-panel"
          aria-selected={activeTab === "examples"}
          className={styles.tab}
          id="image-generation-examples-tab"
          onClick={() => setActiveTab("examples")}
          role="tab"
          type="button"
        >
          {guide.examplesTab}
        </button>
      </ScrollArea>

      {activeTab === "guide" ? (
        <div
          aria-labelledby="image-generation-guide-tab"
          className={styles.panel}
          id="image-generation-guide-panel"
          role="tabpanel"
        >
          <ol className={styles.steps}>
            {guide.steps.map((step, index) => (
              <li className={styles.step} data-testid="image-generation-guide-step" key={step.number}>
                <div className={`${styles.visual} ${index === 2 ? styles.resultVisual : ""}`}>
                  {index === 0 ? <PromptPreview /> : null}
                  {index === 1 ? <PromptPreview compact /> : null}
                  {index === 2 ? <ResultPreview /> : null}
                </div>
                <span className={styles.stepNumber}>{step.number}</span>
                <h3>{step.title}</h3>
                <p>{step.description}</p>
              </li>
            ))}
          </ol>
        </div>
      ) : (
        <div
          aria-labelledby="image-generation-examples-tab"
          className={styles.panel}
          id="image-generation-examples-panel"
          role="tabpanel"
        >
          <MasonryGrid>
            {examples.map((example) => (
              <li key={example.id}>
                <InspirationExampleCard example={example} />
              </li>
            ))}
          </MasonryGrid>
          <div className={`${selectableStyles.actionItems} ${styles.examplesFooter}`}>
            <Link className={`${selectableStyles.control} ${styles.viewMoreExamples}`} href="/app/inspiration">
              {guide.viewMoreExamples}
            </Link>
          </div>
        </div>
      )}
    </section>
  );
}
