import Link from "next/link";

import styles from "./PromptLibraryCta.module.css";

export function PromptLibraryCta() {
  return (
    <div className={styles.banner}>
      <div><p>Идеи для старта</p><h2>Библиотека промптов</h2><span>Изучайте готовые примеры, копируйте удачные формулировки и создавайте собственные варианты.</span></div>
      <Link href="/login?next=/app/inspiration">Смотреть примеры</Link>
    </div>
  );
}
