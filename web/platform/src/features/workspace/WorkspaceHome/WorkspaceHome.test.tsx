import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ push: vi.fn() })),
}));

import { ru } from "@/i18n/ru";
import { WorkspaceConversationListProvider } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";

import { WorkspaceHome } from "./WorkspaceHome";

describe("WorkspaceHome", () => {
  it("uses a normal chat start prompt and keeps image generation on its explicit route", () => {
    const markup = renderToStaticMarkup(
      <WorkspaceConversationListProvider accountId="workspace-home-test-account" initialConversations={[]}>
        <WorkspaceHome />
      </WorkspaceConversationListProvider>,
    );

    expect(markup).toContain(ru.workspace.startTitle);
    expect(markup).toContain(ru.workspace.promptSupport);
    expect(markup).toContain('href="/app/image"');
    expect(markup).toContain('href="/app/models"');
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
    expect(markup).not.toContain('href="/app/image"');
    expect(markup).not.toContain('href="/app/models"');
  });
});
