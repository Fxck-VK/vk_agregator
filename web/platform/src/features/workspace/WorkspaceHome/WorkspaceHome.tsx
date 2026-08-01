import Link from "next/link";

import { ru } from "@/i18n/ru";

import { WorkspacePrompt } from "../WorkspacePrompt/WorkspacePrompt";

import styles from "./WorkspaceHome.module.css";

type WorkspaceSection = keyof typeof ru.workspace.sections;

type WorkspaceHomeProps = {
  section?: WorkspaceSection;
};

export function WorkspaceHome({ section = "home" }: WorkspaceHomeProps) {
  const content = ru.workspace.sections[section];

  if (section === "home") {
    return (
      <section aria-labelledby="workspace-title" className={`${styles.content} ${styles.startScreen}`}>
        <div className={styles.welcome}>
          <p className={styles.eyebrow}>{ru.workspace.eyebrow}</p>
          <h1 id="workspace-title">{ru.workspace.startTitle}</h1>
          <p className={styles.description}>{ru.workspace.startDescription}</p>
        </div>
        <WorkspacePrompt />
        <nav aria-label={ru.workspace.quickActionsLabel} className={styles.quickActions}>
          <Link className={styles.quickAction} href="/app/image">
            <span>{ru.workspace.imageActionTitle}</span>
            <small>{ru.workspace.imageActionDescription}</small>
          </Link>
          <Link className={styles.quickAction} href="/app/models">
            <span>{ru.workspace.modelsActionTitle}</span>
            <small>{ru.workspace.modelsActionDescription}</small>
          </Link>
        </nav>
      </section>
    );
  }

  return (
    <section aria-labelledby="workspace-title" className={styles.content}>
      <p className={styles.eyebrow}>{ru.workspace.eyebrow}</p>
      <h1 id="workspace-title">{content.title}</h1>
      <p className={styles.description}>{content.description}</p>
    </section>
  );
}
