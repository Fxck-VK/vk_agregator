"use client";

import { useMessages, useDictionary } from "@/i18n/LocaleProvider";

import { useCallback, useRef, useState, type ReactNode } from "react";

import { ImageGenerationControls } from "@/features/image-generation/ImageGenerationComposer/ImageGenerationControls";
import { ImageGenerationFeedback } from "@/features/image-generation/ImageGenerationPanel/ImageGenerationFeedback";
import { useImageGeneration } from "@/features/image-generation/ImageGenerationPanel/useImageGeneration";
import type { GenerationModel } from "@/features/models/generation-model-catalog";

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
  const msg = useMessages();
  const t = useDictionary();
  const [selectedModel, setSelectedModel] = useState<GenerationModel | null>(null);
  const selectedImageModel = selectedModel?.category === "images" ? selectedModel : null;
  const [prompt, setPrompt] = useState("");
  const controlsRef = useRef<HTMLDivElement>(null);
  const [exitingControls, setExitingControls] = useState<ExitingControlsSnapshot | null>(null);
  const finishControlsExit = useCallback(() => setExitingControls(null), []);
  const selectInitialTextModel = useCallback((model: GenerationModel | null) => {
    setSelectedModel((current) => current ?? model);
  }, []);
  const generation = useImageGeneration({ access, model: selectedImageModel, promptValue: prompt, onPromptChange: setPrompt });

  return (
    <>
      <div className={styles.composer}>
        <WorkspacePrompt
          access={access}
          chatModelUnavailable={selectedModel === null}
          leadingControls={selectedImageModel === null ? (
            exitingControls === null ? undefined : (
              <ExitingModelControls onComplete={finishControlsExit} snapshot={exitingControls} />
            )
          ) : (
            <div className={styles.modelControls} key={selectedImageModel.id} ref={controlsRef}>
              <ImageGenerationControls {...generation.composerProps} isSubmitting={generation.busy} />
            </div>
          )}
          onPromptChange={generation.stage === "result" ? generation.createAnother : generation.changePrompt}
          promptValue={prompt}
          selectedGenerationModel={selectedModel ?? undefined}
          submitAction={selectedImageModel === null ? undefined : {
            canSubmit: generation.canPrepare,
            disabled: generation.busy,
            label: generation.stage === "preparing" ? t.imageGeneration.preparing : t.imageGeneration.generate,
            onSubmit: () => void generation.prepareImage(),
          }}
          variant="hero"
        />
        <ImageGenerationFeedback generation={generation} showEditorError />
      </div>
      <nav aria-label={msg("workspaceHero.mainFeatures")} className={modelLinksClassName}>
        <FeaturedModelShortcuts
          disabled={generation.busy}
          onSelect={(model) => {
            if (generation.busy || model.id === selectedModel?.id) return;
            setExitingControls(model.category === "text" ? captureExitingControls(controlsRef.current, generation.composerProps) : null);
            generation.reset();
            setSelectedModel(model);
          }}
          onTextModelLoad={selectInitialTextModel}
          selectedModelId={selectedModel?.id ?? null}
        />
        {allModelsLink}
      </nav>
    </>
  );
}
