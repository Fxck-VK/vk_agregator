"use client";

import { useCallback, useRef, useState, type ReactNode } from "react";

import { ImageGenerationControls } from "@/features/image-generation/ImageGenerationComposer/ImageGenerationControls";
import { ImageGenerationFeedback } from "@/features/image-generation/ImageGenerationPanel/ImageGenerationFeedback";
import { useImageGeneration } from "@/features/image-generation/ImageGenerationPanel/useImageGeneration";
import type { ImageModel } from "@/lib/web-api/contracts";
import { ru } from "@/i18n/ru";

import { FeaturedModelShortcuts } from "../FeaturedModelShortcuts/FeaturedModelShortcuts";
import { WorkspacePrompt } from "../WorkspacePrompt/WorkspacePrompt";
import { captureExitingControls, ExitingModelControls, type ExitingControlsSnapshot } from "./ExitingModelControls";

import styles from "./WorkspaceHero.module.css";

type WorkspaceHeroProps = {
  access: "authenticated" | "guest";
  allModelsLink: ReactNode;
  modelLinksClassName: string;
};

export function WorkspaceHero({ access, allModelsLink, modelLinksClassName }: WorkspaceHeroProps) {
  const [selectedModel, setSelectedModel] = useState<ImageModel | null>(null);
  const [prompt, setPrompt] = useState("");
  const controlsRef = useRef<HTMLDivElement>(null);
  const [exitingControls, setExitingControls] = useState<ExitingControlsSnapshot | null>(null);
  const finishControlsExit = useCallback(() => setExitingControls(null), []);
  const generation = useImageGeneration({ access, model: selectedModel, promptValue: prompt, onPromptChange: setPrompt });

  return (
    <>
      <div className={styles.composer}>
        <WorkspacePrompt
          access={access}
          leadingControls={selectedModel === null ? (
            exitingControls === null ? undefined : (
              <ExitingModelControls onComplete={finishControlsExit} snapshot={exitingControls} />
            )
          ) : (
            <div className={styles.modelControls} key={selectedModel.id} ref={controlsRef}>
              <ImageGenerationControls {...generation.composerProps} isSubmitting={generation.busy} />
            </div>
          )}
          onPromptChange={generation.stage === "result" ? generation.createAnother : generation.changePrompt}
          promptValue={prompt}
          submitAction={selectedModel === null ? undefined : {
            canSubmit: generation.canPrepare,
            disabled: generation.busy,
            label: generation.stage === "preparing" ? ru.imageGeneration.preparing : ru.imageGeneration.generate,
            onSubmit: () => void generation.prepareImage(),
          }}
          variant="hero"
        />
        <ImageGenerationFeedback generation={generation} showEditorError />
      </div>
      <nav aria-label="Основные возможности" className={modelLinksClassName}>
        <FeaturedModelShortcuts
          disabled={generation.busy}
          onSelect={(model) => {
            if (generation.busy || model?.id === selectedModel?.id) return;
            setExitingControls(model === null ? captureExitingControls(controlsRef.current, generation.composerProps) : null);
            generation.reset();
            setSelectedModel(model);
          }}
          selectedModelId={selectedModel?.id ?? null}
        />
        {allModelsLink}
      </nav>
    </>
  );
}
