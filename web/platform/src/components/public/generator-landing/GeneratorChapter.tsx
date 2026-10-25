import type { ReactNode } from "react";
import styles from "./GeneratorLanding.module.css";

export function GeneratorChapter({id,eyebrow,title,intro,children}:{id:string;eyebrow:string;title:string;intro?:string;children:ReactNode}) {
  return <section id={id} className={styles.chapter} aria-labelledby={`${id}-title`}><p className={styles.eyebrow}>{eyebrow}</p><h2 id={`${id}-title`} className={styles.chapterTitle}>{title}</h2>{intro?<p className={styles.chapterIntro}>{intro}</p>:null}{children}</section>;
}
