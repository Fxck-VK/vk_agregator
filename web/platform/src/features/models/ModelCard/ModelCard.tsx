import Link from "next/link";

import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { ru } from "@/i18n/ru";
import type { ImageModel } from "@/lib/web-api/contracts";

import { ModelIcon } from "../ModelIcon/ModelIcon";

import styles from "./ModelCard.module.css";

type ModelCardProps = {
  model: ImageModel;
};

export function ModelCard({ model }: ModelCardProps) {
  const prices = Object.values(model.price_by_quality ?? {});
  const minimumPrice = prices.length > 0 ? Math.min(...prices) : null;

  return (
    <article className={styles.card}>
      <ModelIcon className={styles.modelIcon} />
      <div className={styles.heading}>
        <p className={styles.type}>{ru.modelsCatalog.imageTypeLabel}</p>
        <h3>{model.name}</h3>
      </div>
      <ul aria-label={ru.modelsCatalog.qualityFilterLabel} className={styles.qualities}>
        {model.quality_options.map((value) => (
          <li key={value}>{value}</li>
        ))}
      </ul>
      <p className={styles.reference}>
        {model.supports_reference_image
          ? ru.modelsCatalog.referenceSupportedLabel
          : ru.modelsCatalog.referenceUnsupportedLabel}
      </p>
      {minimumPrice !== null ? <CreditAmount className={styles.price} prefix={ru.modelsCatalog.pricePrefix} value={minimumPrice} /> : null}
      <Link
        aria-label={`${ru.modelsCatalog.openGeneratorLabel}: ${model.name}`}
        href={`/app/image?model=${encodeURIComponent(model.id)}`}
        prefetch={false}
      >
        {ru.modelsCatalog.openGeneratorLabel}
      </Link>
    </article>
  );
}
