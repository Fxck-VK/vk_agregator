import Link from "next/link";

import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import selectableStyles from "@/components/ui/selectable-control.module.css";
import { ru } from "@/i18n/ru";
import type { ImageModel } from "@/lib/web-api/contracts";

import { ModelIcon } from "../ModelIcon/ModelIcon";

import { getModelPresentation } from "./model-card-content";
import styles from "./ModelCard.module.css";

export type ModelCardModel = Pick<ImageModel, "id" | "name"> &
  Partial<Pick<ImageModel, "price_by_quality">> & {
    artworkSrc?: string;
    description?: string;
  };

type SharedModelCardProps = {
  className?: string;
  testId?: string;
};

type CatalogueModelCardProps = SharedModelCardProps & {
  interactive?: boolean;
  model: ImageModel;
  onActivate?: never;
  revealed?: boolean;
  selected?: never;
  variant?: "catalog";
};

type SelectorModelCardProps = SharedModelCardProps & {
  model: ModelCardModel;
  onActivate: (model: ModelCardModel, href: string) => void;
  revealed?: never;
  selected: boolean;
  variant: "selector";
};

type ModelCardProps = CatalogueModelCardProps | SelectorModelCardProps;

export function ModelCard(props: Readonly<ModelCardProps>) {
  const { className, model, testId } = props;
  const presentation = getModelPresentation(model);

  if (props.variant === "selector") {
    const classNames = [
      selectableStyles.control,
      styles.selectorCard,
      className,
    ].filter(Boolean).join(" ");

    return (
      <button
        aria-pressed={props.selected}
        className={classNames}
        data-testid={testId}
        onClick={() => props.onActivate(model, presentation.href)}
        type="button"
      >
        <ModelIcon className={styles.selectorIcon} src={presentation.artworkSrc} />
        <span className={styles.selectorCopy}>
          <span className={styles.selectorTitle}>{model.name}</span>
          <span className={styles.selectorDescription}>{presentation.description}</span>
        </span>
      </button>
    );
  }

  const prices = Object.values(model.price_by_quality ?? {});
  const minimumPrice = prices.length > 0 ? Math.min(...prices) : null;
  const classNames = [styles.cardLink, className].filter(Boolean).join(" ");
  const card = (
    <article className={styles.card}>
      <div className={styles.cardTop}>
        <ModelIcon src={presentation.artworkSrc} />
        {minimumPrice !== null ? <CreditAmount className={styles.price} value={minimumPrice} /> : null}
      </div>
      <div className={styles.copy}>
        <h3>{model.name}</h3>
        <p>{presentation.description}</p>
      </div>
    </article>
  );

  if (props.interactive === false) {
    return (
      <div
        className={`${classNames} ${styles.placeholder}`}
        data-interactive="false"
        data-testid={testId ?? "catalog-placeholder-card"}
      >
        {card}
      </div>
    );
  }

  return (
    <Link
      aria-label={`${ru.modelsCatalog.openGeneratorLabel}: ${model.name}`}
      className={classNames}
      data-revealed={props.revealed || undefined}
      data-testid={testId}
      href={presentation.href}
      prefetch={false}
    >
      {card}
    </Link>
  );
}
