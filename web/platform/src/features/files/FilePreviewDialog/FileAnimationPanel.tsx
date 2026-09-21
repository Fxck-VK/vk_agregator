"use client";

import { useMessages } from "@/i18n/LocaleProvider";
import { actionCreditAmount } from "@/i18n/counts";
import { RichMessage } from "@/i18n/RichMessage";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";


import Image from "next/image";
import { useState } from "react";

import {
  type FileModelTask,
  useFileActionModels,
} from "./file-action-models";
import styles from "./FileAnimationPanel.module.css";
import { FileTaskModelSelector } from "./FileTaskModelSelector";

type FileModelActionPanelProps = {
  actionLabel: string;
  panelLabel: string;
  task: Exclude<FileModelTask, "edit">;
  title: string;
  titleIconSrc: string;
};

function FileModelActionPanel({
  actionLabel,
  panelLabel,
  task,
  title,
  titleIconSrc,
}: Readonly<FileModelActionPanelProps>) {
  const msg = useMessages();
  const { models, status } = useFileActionModels(task);
  const [selectedModelId, setSelectedModelId] = useState("");
  const selectedModel = models.find((model) => model.id === selectedModelId) ?? models[0] ?? null;
  const actionButtonLabel = selectedModel === null
    ? msg("fileAnimationPanel.valueUnavailable", { value1: actionLabel })
    : msg("fileAnimationPanel.valueForValue", { value1: actionLabel, value2: actionCreditAmount(msg, selectedModel.cost) });

  return (
    <section aria-label={panelLabel} className={styles.panel}>
      <header className={styles.title}>
        <Image
          alt=""
          aria-hidden="true"
          height={20}
          src={titleIconSrc}
          unoptimized
          width={20}
        />
        <h2>{title}</h2>
      </header>

      <div className={styles.modelControl}>
        <span className={styles.modelLabel}>{msg("fileAnimationPanel.model")}</span>
        <FileTaskModelSelector
          models={models}
          onSelect={setSelectedModelId}
          selectedModelId={selectedModel?.id ?? ""}
          status={status}
          task={task}
        />
      </div>

      <button
        aria-label={actionButtonLabel}
        className={styles.actionButton}
        disabled
        type="button"
      >
        {selectedModel === null ? (
          actionButtonLabel
        ) : (
          <RichMessage id="fileAnimationPanel.valueForValue" values={{ value1: actionLabel, value2: <CreditAmount aria-hidden="true" value={selectedModel.cost} /> }} />
        )}
      </button>
    </section>
  );
}

export function FileAnimationPanel() {
  const msg = useMessages();
  return (
    <FileModelActionPanel
      actionLabel={msg("fileAnimationPanel.animate")}
      panelLabel={msg("fileAnimationPanel.animationSettings")}
      task="animate"
      title={msg("fileAnimationPanel.animate")}
      titleIconSrc="/assets/icons/ui/animate-white.svg"
    />
  );
}

export function FileEnhancementPanel() {
  const msg = useMessages();
  return (
    <FileModelActionPanel
      actionLabel={msg("fileAnimationPanel.enhance")}
      panelLabel={msg("fileAnimationPanel.enhancementSettings")}
      task="enhance"
      title={msg("fileAnimationPanel.enhance")}
      titleIconSrc="/assets/icons/ui/enhance-white.svg"
    />
  );
}

export function FileBackgroundRemovalPanel() {
  const msg = useMessages();
  return (
    <FileModelActionPanel
      actionLabel={msg("fileAnimationPanel.removeBackground")}
      panelLabel={msg("fileAnimationPanel.backgroundRemovalSettings")}
      task="remove-background"
      title={msg("fileAnimationPanel.removeBackground")}
      titleIconSrc="/assets/icons/ui/remove-background-white.svg"
    />
  );
}
