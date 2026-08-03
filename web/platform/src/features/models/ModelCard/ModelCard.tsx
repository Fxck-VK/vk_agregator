import Link from "next/link";

import { ru } from "@/i18n/ru";
import type { ImageModel } from "@/lib/web-api/contracts";

import styles from "./ModelCard.module.css";

type ModelCardProps = {
  model: ImageModel;
};

export function ModelCard({ model }: ModelCardProps) {
  const prices = Object.values(model.price_by_quality ?? {});
  const minimumPrice = prices.length > 0 ? Math.min(...prices) : null;

  return (
    <article className={styles.card}>
      <div className={styles.heading}>
        <p className={styles.type}>{ru.modelsCatalog.imageTypeLabel}</p>
        <h2>{model.name}</h2>
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
      {minimumPrice !== null ? <p>{`От ${minimumPrice} ★`}</p> : null}
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
