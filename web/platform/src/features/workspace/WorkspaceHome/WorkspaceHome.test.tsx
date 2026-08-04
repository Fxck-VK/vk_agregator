import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ push: vi.fn() })),
}));

import { ru } from "@/i18n/ru";
import { WorkspaceConversationListProvider } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";

import { WorkspaceHome } from "./WorkspaceHome";

describe("WorkspaceHome", () => {
  it("renders the complete NeiroHub overview only on the workspace home route", () => {
    const markup = renderToStaticMarkup(
      <WorkspaceConversationListProvider accountId="workspace-home-test-account" initialConversations={[]}>
        <WorkspaceHome />
      </WorkspaceConversationListProvider>,
    );
    const text = markup.replace(/<[^>]+>/g, "");

    expect(text).toContain("Простой старт в мир нейросетей");
    expect(markup).toContain("Новости");
    expect(markup).toContain("Нейросети для разных задач");
    expect(markup).toContain("Как работает NeiroHub");
    expect(markup).toContain("Откройте новые возможности");
    expect(markup).toContain("Ваш план");
    expect(markup).toContain("Библиотека промптов");
    expect(markup).toContain("Частые вопросы");
    expect(markup).toContain("Сообщество NeiroHub");
    expect(markup).toContain('href="/app/image"');
    expect(markup).toContain('href="/app/models"');
    expect(markup).toContain('href="/app/inspiration"');
    expect(markup).toContain('href="/app/profile"');
    expect(markup).not.toContain("<main");
    expect(markup).not.toContain("image-generation-title");
    expect(markup).not.toContain("image-job-history-title");
  });

  it("renders the interactive inspiration example instead of a placeholder", () => {
    const markup = renderToStaticMarkup(<WorkspaceHome section="inspiration" />);

    expect(markup).toContain(ru.inspiration.title);
    expect(markup).toContain(ru.inspiration.openExample);
    expect(markup).toContain("%2Finspiration%2Fpaper-crane-cloud.png");
    expect(markup).not.toContain(ru.workspace.sections.inspiration.description);
  });

  it("renders the normal-chat entry instead of the old chats placeholder", () => {
    const markup = renderToStaticMarkup(
      <WorkspaceConversationListProvider accountId="new-chat-test-account" initialConversations={[]}>
        <WorkspaceHome section="chats" />
      </WorkspaceConversationListProvider>,
    );

    expect(markup).toContain(ru.workspace.startTitle);
    expect(markup).toContain(ru.conversations.composerPlaceholder);
    expect(markup).not.toContain(ru.workspace.sections.chats.title);
    expect(markup).not.toContain(ru.workspace.sections.chats.description);
    expect(markup).not.toContain("Нейросети для разных задач");
    expect(markup).not.toContain("Частые вопросы");
    expect(markup).not.toContain('href="/app/image"');
    expect(markup).not.toContain('href="/app/models"');
  });
});
