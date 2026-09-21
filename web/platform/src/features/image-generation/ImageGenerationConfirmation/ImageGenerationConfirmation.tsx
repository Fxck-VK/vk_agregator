"use client";

import { StateNotice, LoadingIndicator } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary } from "@/i18n/LocaleProvider";


import { Button } from "@/components/ui/Button/Button";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
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
  const t = useDictionary();
  const balanceAfter = Math.max(0, preparation.balance - preparation.job.cost_estimate);

  return (
    <section aria-labelledby="image-confirmation-title" className={styles.confirmation}>
      <h3 id="image-confirmation-title">{t.imageGeneration.confirmationTitle}</h3>
      <dl>
        <div>
          <dt>{t.imageGeneration.costLabel}</dt>
          <dd><CreditAmount value={preparation.job.cost_estimate} /></dd>
        </div>
        <div>
          <dt>{t.imageGeneration.balanceLabel}</dt>
          <dd><CreditAmount value={preparation.balance} /></dd>
        </div>
        <div>
          <dt>{t.imageGeneration.balanceAfterLabel}</dt>
          <dd><CreditAmount value={balanceAfter} /></dd>
        </div>
      </dl>
      <Button variant="outline" disabled={isActivating} onClick={onConfirm}>
        {isActivating ? (
          <><LoadingIndicator label={t.imageGeneration.activating} />{t.imageGeneration.activating}</>
        ) : (
          <>{t.imageGeneration.confirm} · <CreditAmount value={preparation.job.cost_estimate} /></>
        )}
      </Button>
      {errorMessage !== null ? (
        <StateNotice inline kind="error">
          {errorMessage}
        </StateNotice>
      ) : null}
    </section>
  );
}
