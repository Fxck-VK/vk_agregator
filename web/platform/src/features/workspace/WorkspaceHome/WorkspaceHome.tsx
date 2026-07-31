import Link from "next/link";

import { NewConversationButton } from "@/features/conversations/NewConversationButton/NewConversationButton";
import { ru } from "@/i18n/ru";

import styles from "./WorkspaceHome.module.css";

type WorkspaceSection = keyof typeof ru.workspace.sections;

type WorkspaceHomeProps = {
  section?: WorkspaceSection;
};

export function WorkspaceHome({ section = "home" }: WorkspaceHomeProps) {
  const content = ru.workspace.sections[section];

  return (
    <section aria-labelledby="workspace-title" className={styles.content}>
      <p className={styles.eyebrow}>{ru.workspace.eyebrow}</p>
      <h1 id="workspace-title">{content.title}</h1>
      <p className={styles.description}>{content.description}</p>
      {section === "home" ? (
        <div className={styles.quickStart}>
          <div>
            <h2>{ru.workspace.quickStartTitle}</h2>
            <p>{ru.workspace.quickStartDescription}</p>
          </div>
          <div className={styles.actions}>
            <NewConversationButton />
            <Link href="/app/models">{ru.workspace.openModels}</Link>
          </div>
        </div>
      ) : null}
    </section>
  );
}
