import { ru } from "@/i18n/ru";
import { InspirationGallery } from "@/features/inspiration/InspirationGallery/InspirationGallery";

import { WorkspaceLanding } from "../WorkspaceLanding/WorkspaceLanding";
import { WorkspacePrompt } from "../WorkspacePrompt/WorkspacePrompt";
import { NewChatPrompt } from "../WorkspacePrompt/NewChatPrompt";

import styles from "./WorkspaceHome.module.css";

type WorkspaceSection = keyof typeof ru.workspace.sections;

type WorkspaceHomeProps = {
  access?: "authenticated" | "guest";
  section?: WorkspaceSection;
  chatModelId?: string;
};

export function WorkspaceHome({ access = "authenticated", section = "home", chatModelId }: WorkspaceHomeProps) {
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
          {chatModelId && access === "authenticated"
            ? <NewChatPrompt modelId={chatModelId} />
            : <WorkspacePrompt access={access} variant="newChat" />}
        </div>
      </section>
    );
  }

  return (
    <section aria-labelledby="workspace-title" className={styles.content}>
      <h1 id="workspace-title">{content.title}</h1>
      <p className={styles.description}>{content.description}</p>
    </section>
  );
}
