"use client";

import Link from "next/link";
import { useState } from "react";

import { landingModels } from "../landing-content";
import { PublicModelCard } from "../PublicModelCard/PublicModelCard";
import styles from "./PopularModels.module.css";

export function PopularModels() {
  const [expanded, setExpanded] = useState(false);
  const models = expanded ? landingModels : landingModels.slice(0, 4);

  return (
    <div>
      <div className={styles.heading}>
        <div><h2 id="models-title">Популярные нейросети</h2><p>Выберите готовый инструмент для текста, изображений, анализа и других задач.</p></div>
        <Link href="/login?next=/app/models">Открыть каталог</Link>
      </div>
      <div className={styles.grid}>{models.map((model) => <PublicModelCard key={model.id} model={model} />)}</div>
      {!expanded ? <button className={styles.more} onClick={() => setExpanded(true)} type="button">Показать ещё</button> : null}
    </div>
  );
}
