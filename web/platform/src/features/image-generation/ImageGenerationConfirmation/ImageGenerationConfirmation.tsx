"use client";

import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";
import type { ImageJobPreparation } from "@/lib/web-api/contracts";

import styles from "./ImageGenerationConfirmation.module.css";

type ImageGenerationConfirmationProps = {
  errorMessage: string | null;
  isActivating: boolean;
  onConfirm: () => void;
  preparation: ImageJobPreparation;
};

export function ImageGenerationConfirmation({
  errorMessage,
  isActivating,
  onConfirm,
  preparation,
}: Readonly<ImageGenerationConfirmationProps>) {
  const balanceAfter = Math.max(0, preparation.balance - preparation.job.cost_estimate);

  return (
    <section aria-labelledby="image-confirmation-title" className={styles.confirmation}>
      <h3 id="image-confirmation-title">{ru.imageGeneration.confirmationTitle}</h3>
      <dl>
        <div>
          <dt>{ru.imageGeneration.costLabel}</dt>
          <dd>{formatStars(preparation.job.cost_estimate)}</dd>
        </div>
        <div>
          <dt>{ru.imageGeneration.balanceLabel}</dt>
          <dd>{formatStars(preparation.balance)}</dd>
        </div>
        <div>
          <dt>{ru.imageGeneration.balanceAfterLabel}</dt>
          <dd>{formatStars(balanceAfter)}</dd>
        </div>
      </dl>
      <Button disabled={isActivating} onClick={onConfirm}>
        {isActivating ? ru.imageGeneration.activating : `${ru.imageGeneration.confirm} · ${formatStars(preparation.job.cost_estimate)}`}
      </Button>
      {errorMessage !== null ? (
        <p className={styles.error} role="alert">
          {errorMessage}
        </p>
      ) : null}
    </section>
  );
}

function formatStars(value: number): string {
  return `${value} ★`;
}
