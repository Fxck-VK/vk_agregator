import { renderToStaticMarkup } from "react-dom/server";
import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ push: vi.fn() })),
}));

vi.mock("@/features/models/image-model-catalog-cache", () => ({
  loadImageModelCatalog: vi.fn(),
}));

import { ru } from "@/i18n/ru";
import { WorkspaceConversationListProvider } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { loadImageModelCatalog } from "@/features/models/image-model-catalog-cache";

import { WorkspaceHome } from "./WorkspaceHome";

describe("WorkspaceHome", () => {
  it("omits the standalone NeiroHub eyebrow from generic section headings", () => {
    const markup = renderToStaticMarkup(<WorkspaceHome section="models" />);

    expect(markup).toContain(ru.workspace.sections.models.title);
    expect(markup).not.toMatch(/<p[^>]*>NeiroHub<\/p>/);
  });

  it("renders the complete NeiroHub overview only on the workspace home route", () => {
    const markup = renderToStaticMarkup(
      <WorkspaceConversationListProvider accountId="workspace-home-test-account" initialConversations={[]}>
        <WorkspaceHome />
      </WorkspaceConversationListProvider>,
    );
    const text = markup.replace(/<[^>]+>/g, "");

    expect(text).toContain("Простой старт в мир нейросетей");
    expect(markup).not.toContain("Новости");
    expect(markup).not.toContain("Всё нужное для работы с AI — рядом");
    expect(markup).toContain("Популярные нейросети");
    expect(markup).not.toContain("Один аккаунт — разные сценарии");
    expect(markup).not.toContain("Нейросети для разных задач");
    expect(markup).toContain("Как работает NeiroHub");
    expect(markup).toContain("Видео скоро появится");
    expect(markup).not.toContain("Сформулируйте задачу");
    expect(markup).not.toContain("<video");
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

  it("renders four compact model cards from truthful catalogue data", async () => {
    vi.mocked(loadImageModelCatalog).mockResolvedValue({
      items: [
        {
          id: "nano-banana-2",
          name: "Nano Banana 2",
          quality_options: ["1K", "2K"],
          price_by_quality: { "1K": 55, "2K": 70 },
          default_quality: "1K",
          supports_reference_image: true,
          max_reference_images: 4,
        },
        {
          id: "gpt-image-2",
          name: "GPT Image 2",
          quality_options: ["1K"],
          price_by_quality: { "1K": 51 },
          default_quality: "1K",
          supports_reference_image: false,
          max_reference_images: 0,
        },
        {
          id: "seedream-5-pro",
          name: "Seedream 5.0 Pro",
          quality_options: ["2K"],
          price_by_quality: { "2K": 35 },
          default_quality: "2K",
          supports_reference_image: true,
          max_reference_images: 2,
        },
        {
          id: "midjourney-v7",
          name: "Midjourney",
          quality_options: ["standard"],
          price_by_quality: { standard: 55 },
          default_quality: "standard",
          supports_reference_image: true,
          max_reference_images: 1,
        },
        {
          id: "hidden-fifth-model",
          name: "Пятая модель",
          quality_options: ["1K"],
          price_by_quality: { "1K": 99 },
          default_quality: "1K",
          supports_reference_image: false,
          max_reference_images: 0,
        },
      ],
    });

    render(
      <WorkspaceConversationListProvider accountId="featured-models-test-account" initialConversations={[]}>
        <WorkspaceHome />
      </WorkspaceConversationListProvider>,
    );

    const region = await screen.findByRole("region", { name: "Популярные нейросети" });
    const cards = within(region).getAllByTestId("featured-model-card");
    const shortcutsNavigation = screen.getByRole("navigation", { name: "Основные возможности" });
    const shortcuts = within(shortcutsNavigation).getAllByTestId("featured-model-shortcut");
    const shortcutLinks = within(shortcutsNavigation).getAllByRole("link");

    expect(cards).toHaveLength(4);
    expect(cards[0]).toHaveAttribute("href", "/app/image?model=nano-banana-2");
    expect(cards[0]).toHaveTextContent("Nano Banana 2");
    expect(within(cards[0]).getByLabelText("от 55 звёзд")).toBeInTheDocument();
    expect(cards[0]).toHaveTextContent("1K");
    expect(cards[0]).toHaveTextContent("Поддерживает референсы");
    expect(within(region).queryByText("Пятая модель")).toBeNull();
    expect(within(region).getAllByTestId("model-icon-fallback")).toHaveLength(4);
    expect(within(region).queryByTestId("model-icon-placeholder")).toBeNull();
    expect(region).not.toHaveTextContent("Открыть");
    expect(region).not.toHaveTextContent("рейтинг");
    expect(region).not.toHaveTextContent("запусков");
    expect(shortcuts).toHaveLength(4);
    expect(shortcuts.map((shortcut) => shortcut.textContent)).toEqual([
      "Nano Banana 2",
      "GPT Image 2",
      "Seedream 5.0 Pro",
      "Midjourney",
    ]);
    expect(shortcuts[0]).toHaveAttribute("href", "/app/image?model=nano-banana-2");
    expect(shortcutsNavigation).not.toHaveTextContent("NeiroHub Chat");
    expect(shortcutsNavigation).not.toHaveTextContent("Генератор изображений");
    expect(shortcutsNavigation).not.toHaveTextContent("Каталог нейросетей");
    expect(shortcutsNavigation).not.toHaveTextContent("Вдохновение");
    expect(shortcutLinks.at(-1)).toHaveTextContent("Все нейросети");
    expect(shortcutLinks.at(-1)).toHaveAttribute("href", "/app/models");
  });

  it("renders the interactive inspiration example instead of a placeholder", () => {
    const markup = renderToStaticMarkup(<WorkspaceHome section="inspiration" />);

    expect(markup).toContain(ru.inspiration.title);
    expect(markup).toContain(ru.inspiration.openExample);
    expect(markup).toContain("%2Fassets%2Fimages%2Finspiration%2Fpaper-crane-cloud.png");
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

  it("keeps the new-chat heading and composer in one centered layout group", () => {
    render(
      <WorkspaceConversationListProvider accountId="new-chat-layout-test-account" initialConversations={[]}>
        <WorkspaceHome section="chats" />
      </WorkspaceConversationListProvider>,
    );

    const layout = screen.getByRole("group", { name: ru.workspace.startTitle });

    expect(layout).toContainElement(screen.getByRole("heading", { name: ru.workspace.startTitle }));
    expect(layout).toContainElement(screen.getByRole("textbox", { name: ru.conversations.composerPlaceholder }));
  });
});
