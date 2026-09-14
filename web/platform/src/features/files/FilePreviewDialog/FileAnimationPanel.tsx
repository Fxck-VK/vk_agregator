"use client";

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

function CreditStar() {
  return (
    <Image
      alt=""
      aria-hidden="true"
      className={styles.creditStar}
      height={18}
      src="/assets/icons/ui/star-white.svg"
      unoptimized
      width={18}
    />
  );
}

function getActionCreditAmountLabel(value: number): string {
  const absoluteValue = Math.abs(value);
  const lastTwoDigits = absoluteValue % 100;
  const lastDigit = absoluteValue % 10;

  if (lastTwoDigits >= 11 && lastTwoDigits <= 14) return `${value} звёзд`;
  if (lastDigit === 1) return `${value} звезду`;
  if (lastDigit >= 2 && lastDigit <= 4) return `${value} звезды`;
  return `${value} звёзд`;
}

function FileModelActionPanel({
  actionLabel,
  panelLabel,
  task,
  title,
  titleIconSrc,
}: Readonly<FileModelActionPanelProps>) {
  const { models, status } = useFileActionModels(task);
  const [selectedModelId, setSelectedModelId] = useState("");
  const selectedModel = models.find((model) => model.id === selectedModelId) ?? models[0] ?? null;
  const actionButtonLabel = selectedModel === null
    ? `${actionLabel} недоступно`
    : `${actionLabel} за ${getActionCreditAmountLabel(selectedModel.cost)}`;

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
        <span className={styles.modelLabel}>Модель</span>
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
          <>
            {actionLabel} за {selectedModel.cost}
            <CreditStar />
          </>
        )}
      </button>
    </section>
  );
}

export function FileAnimationPanel() {
  return (
    <FileModelActionPanel
      actionLabel="Оживить"
      panelLabel="Настройки анимации"
      task="animate"
      title="Оживить"
      titleIconSrc="/assets/icons/ui/animate-white.svg"
    />
  );
}

export function FileEnhancementPanel() {
  return (
    <FileModelActionPanel
      actionLabel="Улучшить"
      panelLabel="Настройки улучшения"
      task="enhance"
      title="Улучшить"
      titleIconSrc="/assets/icons/ui/enhance-white.svg"
    />
  );
}

export function FileBackgroundRemovalPanel() {
  return (
    <FileModelActionPanel
      actionLabel="Удалить фон"
      panelLabel="Настройки удаления фона"
      task="remove-background"
      title="Удалить фон"
      titleIconSrc="/assets/icons/ui/remove-background-white.svg"
    />
  );
}
