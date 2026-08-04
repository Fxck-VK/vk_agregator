import Link from "next/link";

import type { LandingModel } from "../landing-contracts";
import styles from "./PublicModelCard.module.css";

export function PublicModelCard({ model }: { model: LandingModel }) {
  return (
    <Link className={styles.card} data-testid="public-model-card" href={model.href}>
      <span aria-hidden="true" className={styles.icon}>{model.icon}</span>
      {model.priceStars ? <span className={styles.price}>{model.priceStars} ★</span> : null}
      <h3>{model.name}</h3>
      <p>{model.description}</p>
      <span className={styles.open}>Открыть <span aria-hidden="true">→</span></span>
    </Link>
  );
}
