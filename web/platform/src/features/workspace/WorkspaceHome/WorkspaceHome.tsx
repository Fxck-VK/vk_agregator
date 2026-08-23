import { ru } from "@/i18n/ru";
import { InspirationGallery } from "@/features/inspiration/InspirationGallery/InspirationGallery";

import { WorkspaceLanding } from "../WorkspaceLanding/WorkspaceLanding";
import { WorkspacePrompt } from "../WorkspacePrompt/WorkspacePrompt";

import styles from "./WorkspaceHome.module.css";

type WorkspaceSection = keyof typeof ru.workspace.sections;

type WorkspaceHomeProps = {
  access?: "authenticated" | "guest";
  section?: WorkspaceSection;
};

export function WorkspaceHome({ access = "authenticated", section = "home" }: WorkspaceHomeProps) {
  const content = ru.workspace.sections[section];

  if (section === "inspiration") {
    return <InspirationGallery />;
  }

  if (section === "home") {
    return <WorkspaceLanding access={access} />;
  }

  if (section === "chats") {
    return (
      <section
        aria-labelledby="new-chat-title"
        className={`${styles.content} ${styles.startScreen} ${styles.newChatScreen}`}
      >
        <div aria-labelledby="new-chat-title" className={styles.newChatContent} role="group">
          <div className={styles.welcome}>
            <h1 id="new-chat-title">{ru.workspace.startTitle}</h1>
          </div>
          <WorkspacePrompt access={access} variant="newChat" />
        </div>
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
