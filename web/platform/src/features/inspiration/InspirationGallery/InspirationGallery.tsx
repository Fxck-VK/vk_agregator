import { ru } from "@/i18n/ru";

import { InspirationExampleCard } from "../InspirationExampleCard/InspirationExampleCard";
import { inspirationExamples } from "../inspiration-examples";
import styles from "./InspirationGallery.module.css";

export function InspirationGallery() {
  return (
    <section aria-labelledby="inspiration-title" className={styles.gallery}>
      <div className={styles.heading}>
        <p className={styles.eyebrow}>{ru.inspiration.eyebrow}</p>
        <h1 id="inspiration-title">{ru.inspiration.title}</h1>
        <p>{ru.inspiration.description}</p>
      </div>

      <div className={styles.grid}>
        {inspirationExamples.map((example, index) => (
          <InspirationExampleCard example={example} key={example.id} priority={index === 0} />
        ))}
      </div>
    </section>
  );
}
