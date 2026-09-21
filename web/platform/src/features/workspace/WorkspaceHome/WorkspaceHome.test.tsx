import { renderToStaticMarkup } from "react-dom/server";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ push: vi.fn() })),
}));

vi.mock("@/features/models/generation-model-catalog", () => ({
  loadGenerationModelCatalog: vi.fn(),
}));

import { ru } from "@/i18n/ru";
import { WorkspaceConversationListProvider } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { inspirationExamples } from "@/features/inspiration/inspiration-examples";
import { loadGenerationModelCatalog } from "@/features/models/generation-model-catalog";
import { imageModelFixture } from "@/test/model-catalog";

import { WorkspaceHome } from "./WorkspaceHome";

const chatModel = {
  id: "neirohub-chat",
  name: "NeiroHub Chat",
  description: "Текстовая модель из общего каталога",
  category: "text" as const,
  categories: ["popular", "text", "study-work", "free"],
  estimate_credits: 0,
  isFree: true,
};
function imageGenerationModel(model: Parameters<typeof imageModelFixture>[0]) {
  return { ...imageModelFixture(model), category: "images" as const };
}
function generationCatalog(images: ReturnType<typeof imageGenerationModel>[] = []) {
  return {
    default_model_id: chatModel.id,
    categoryErrors: {},
    items: [...images, chatModel],
  } as Awaited<ReturnType<typeof loadGenerationModelCatalog>>;
}

describe("WorkspaceHome", () => {
  beforeEach(() => {
    vi.mocked(loadGenerationModelCatalog).mockResolvedValue(generationCatalog());
  });

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
    expect(markup).not.toContain("Видео скоро появится");
    expect(markup).not.toContain("Сформулируйте задачу");
    expect(markup).toContain("<video");
    expect(markup).not.toContain(" controls");
    expect(markup).toContain('preload="none"');
    expect(markup).toContain('poster="/assets/images/workspace/neirohub-how-it-works-poster.png"');
    expect(markup).toContain('src="/assets/videos/workspace/neirohub-how-it-works.mp4"');
    expect(markup).toContain('type="video/mp4"');
    expect(markup).toContain('aria-label="Воспроизвести: Как работает NeiroHub"');
    expect(markup).toContain("Откройте новые возможности");
    expect(markup).toContain("Lite");
    expect(markup).toContain("400");
    expect(markup).toContain("199");
    expect(markup).toContain("₽/нед");
    expect(markup).toContain("Выбрать тариф");
    expect(markup).toContain("до 13 генераций изображений");
    expect(markup).toContain("до 2 генераций видео");
    expect(markup).toContain("Доступ к популярным нейросетям");
    expect(markup).toContain('src="/assets/icons/ui/image-generations-white.svg"');
    expect(markup).toContain('src="/assets/icons/ui/video-generations-white.svg"');
    expect(markup).toContain('src="/assets/icons/ui/ai-access-white.svg"');
    expect(markup).not.toContain("Ваш план");
    expect(markup).not.toContain("Открыть профиль");
    expect(markup).toContain("Библиотека промптов");
    expect(markup).toContain("Частые вопросы");
    expect(markup.match(/<details/g)).toHaveLength(5);
    for (const question of [
      "Что такое NeiroHub?",
      "Что такое собственные нейросети NeiroHub?",
      "Что такое токены и подписка?",
      "Как купить подписку?",
      "Есть ли бесплатный доступ?",
    ]) {
      expect(text).toContain(question);
    }
    for (const removedQuestion of [
      "Где сохраняются мои диалоги?",
      "Как рассчитывается стоимость генерации?",
      "Где найти созданные изображения?",
      "Можно ли пользоваться с телефона?",
    ]) {
      expect(text).not.toContain(removedQuestion);
    }
    expect(markup).toContain("Следи за нами в Telegram и VK");
    expect(markup).toContain("Будь в тренде и работай с AI быстрее");
    expect(markup).toContain('aria-disabled="true"');
    expect(markup).toContain(">Telegram</span>");
    expect(markup).toContain('href="https://vk.me/neirohub_help"');
    expect(markup).toContain(">ВКонтакте</a>");
    expect(markup).not.toContain("Сообщество NeiroHub");
    expect(markup).not.toContain("Перейти во вдохновение");
    expect(markup).not.toContain("Открыть мои файлы");
    expect(markup).toContain('href="/ru/app/image"');
    expect(markup).toContain('href="/ru/app/models"');
    expect(markup).toContain('href="/ru/app/inspiration"');
    expect(markup).toContain('href="/ru/app/profile"');
    expect(markup).not.toContain("<main");
    expect(markup).not.toContain("image-generation-title");
    expect(markup).not.toContain("image-job-history-title");
  });

  it("omits the editorial kicker labels", () => {
    const markup = renderToStaticMarkup(
      <WorkspaceConversationListProvider accountId="workspace-kickers-test-account" initialConversations={[]}>
        <WorkspaceHome />
      </WorkspaceConversationListProvider>,
    );

    for (const removedLabel of [
      "Коротко о главном",
      "Не только обычный чат",
      "Начните с готовой идеи",
      "Помощь по платформе",
      "Идеи и примеры",
      "Аккаунт и баланс",
    ]) {
      expect(markup).not.toContain(removedLabel);
    }
  });

  it("renders one shared prompt-library card and opens its existing dialog", () => {
    render(
      <WorkspaceConversationListProvider accountId="prompt-library-test-account" initialConversations={[]}>
        <WorkspaceHome />
      </WorkspaceConversationListProvider>,
    );

    expect(screen.getByText("Собрали промпты для любых задач и идей")).toBeInTheDocument();
    for (const removedText of [
      "Все идеи",
      "Промпт для изображения",
      "Воздушная бумажная скульптура среди мягких облаков",
      "Посмотреть пример",
    ]) {
      expect(screen.queryByText(removedText)).not.toBeInTheDocument();
    }

    const promptCards = screen.getAllByRole("button", { name: inspirationExamples[0].openLabel });

    expect(promptCards).toHaveLength(1);
    fireEvent.click(promptCards[0]);
    expect(screen.getByRole("dialog", { name: ru.inspiration.dialogLabel })).toBeInTheDocument();
  });

  it("opens the subscription plans dialog from the landing plan card without navigating", () => {
    render(
      <WorkspaceConversationListProvider accountId="landing-plan-test-account" initialConversations={[]}>
        <WorkspaceHome />
      </WorkspaceConversationListProvider>,
    );

    const tariffButton = screen.getByRole("button", { name: "Выбрать тариф" });

    expect(tariffButton).toHaveAttribute("type", "button");
    expect(tariffButton).not.toHaveAttribute("href");
    fireEvent.click(tariffButton);
    expect(screen.getByRole("dialog", { name: "С подпиской — максимум возможностей" })).toBeInTheDocument();
  });

  it("reveals two additional compact model cards before linking to the catalogue", async () => {
    vi.mocked(loadGenerationModelCatalog).mockResolvedValue(generationCatalog([
      imageGenerationModel({
        id: "nano-banana-2",
        name: "Nano Banana 2",
        description: "Быстрая генерация и редактирование изображений для повседневных задач",
        quality_options: ["1K", "2K"],
        price_by_quality: { "1K": 55, "2K": 70 },
        default_quality: "1K",
        supports_reference_image: true,
        max_reference_images: 4,
        categories: ["popular", "images"],
      }),
      imageGenerationModel({
        id: "nano-banana-pro",
        name: "Nano Banana Pro",
        description: "Детализированные изображения для сложных творческих и рабочих задач",
        quality_options: ["1K"],
        price_by_quality: { "1K": 50 },
        default_quality: "1K",
        supports_reference_image: true,
        max_reference_images: 4,
        categories: ["popular", "images"],
      }),
      imageGenerationModel({
        id: "gpt-image-2",
        name: "GPT Image 2",
        description: "Точное создание изображений по описанию с хорошей передачей текста",
        quality_options: ["1K"],
        price_by_quality: { "1K": 40 },
        default_quality: "1K",
        supports_reference_image: false,
        max_reference_images: 0,
        categories: ["popular", "images"],
      }),
      imageGenerationModel({
        id: "seedream-4.5",
        name: "Seedream 4.5",
        description: "Фотореалистичные изображения с высокой детализацией и выразительным стилем",
        quality_options: ["2K", "4K"],
        price_by_quality: { "2K": 30, "4K": 45 },
        default_quality: "2K",
        supports_reference_image: true,
        max_reference_images: 2,
        categories: ["popular", "images"],
      }),
      imageGenerationModel({
        id: "hidden-fifth-model",
        name: "Пятая модель",
        quality_options: ["1K"],
        price_by_quality: { "1K": 99 },
        default_quality: "1K",
        supports_reference_image: false,
        max_reference_images: 0,
        categories: ["popular", "images"],
      }),
      imageGenerationModel({
        id: "hidden-sixth-model",
        name: "Шестая модель",
        quality_options: ["1K"],
        price_by_quality: { "1K": 109 },
        default_quality: "1K",
        supports_reference_image: false,
        max_reference_images: 0,
        categories: ["popular", "images"],
      }),
    ]));

    render(
      <WorkspaceConversationListProvider accountId="featured-models-test-account" initialConversations={[]}>
        <WorkspaceHome />
      </WorkspaceConversationListProvider>,
    );

    const region = await screen.findByRole("region", { name: "Популярные нейросети" });
    let cards = within(region).getAllByTestId("featured-model-card");
    const shortcutsNavigation = screen.getByRole("navigation", { name: "Основные возможности" });
    const shortcuts = within(shortcutsNavigation).getAllByTestId("featured-model-shortcut");
    const shortcutLinks = within(shortcutsNavigation).getAllByRole("link");

    expect(cards).toHaveLength(4);
    expect(cards[0]).toHaveAttribute("href", "/ru/app/chats?model=nano-banana-2");
    expect(cards[0]).toHaveTextContent("Nano Banana 2");
    expect(within(cards[0]).getByLabelText("55 звёзд")).toBeInTheDocument();
    expect(within(cards[0]).queryByLabelText("от 55 звёзд")).toBeNull();
    expect(cards[0]).toHaveTextContent("Быстрая генерация и редактирование изображений для повседневных задач");
    expect(cards[1]).toHaveTextContent("Детализированные изображения для сложных творческих и рабочих задач");
    expect(cards[2]).toHaveTextContent("Точное создание изображений по описанию с хорошей передачей текста");
    expect(cards[3]).toHaveTextContent("Фотореалистичные изображения с высокой детализацией и выразительным стилем");
    expect(within(region).queryByText("Пятая модель")).toBeNull();
    expect(within(region).queryByText("Шестая модель")).toBeNull();
    expect(within(region).queryByRole("link", { name: "Все нейросети" })).toBeNull();

    fireEvent.click(within(region).getByRole("button", { name: "Показать ещё" }));

    cards = within(region).getAllByTestId("featured-model-card");
    expect(cards).toHaveLength(6);
    expect(cards[4]).toHaveTextContent("Пятая модель");
    expect(cards[5]).toHaveTextContent("Шестая модель");
    expect(within(region).getByRole("link", { name: "Все нейросети" })).toHaveAttribute("href", "/ru/app/models");
    expect(region).not.toHaveTextContent("1K");
    expect(region).not.toHaveTextContent("2K");
    expect(region).not.toHaveTextContent("4K");
    expect(region).not.toHaveTextContent("Поддерживает референсы");
    expect(region).not.toHaveTextContent("По текстовому запросу");
    expect(within(region).getAllByTestId("model-icon-fallback")).toHaveLength(6);
    expect(within(region).queryByTestId("model-icon-placeholder")).toBeNull();
    expect(region).not.toHaveTextContent("Открыть");
    expect(region).not.toHaveTextContent("рейтинг");
    expect(region).not.toHaveTextContent("запусков");
    expect(shortcuts).toHaveLength(4);
    expect(shortcuts.map((shortcut) => shortcut.textContent)).toEqual([
      "Nano Banana 2",
      "Nano Banana Pro",
      "GPT Image 2",
      "Seedream 4.5",
    ]);
    expect(shortcuts[0]).not.toHaveAttribute("href");
    expect(within(shortcutsNavigation).getByRole("button", { name: "Выбрать модель: NeiroHub Chat" })).toHaveAttribute("aria-pressed", "true");
    expect(shortcutsNavigation).not.toHaveTextContent("Генератор изображений");
    expect(shortcutsNavigation).not.toHaveTextContent("Каталог нейросетей");
    expect(shortcutsNavigation).not.toHaveTextContent("Вдохновение");
    expect(shortcutLinks.at(-1)).toHaveTextContent("Все нейросети");
    expect(shortcutLinks.at(-1)).toHaveAttribute("href", "/ru/app/models");
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
    expect(markup).not.toContain('href="/ru/app/image"');
    expect(markup).not.toContain('href="/ru/app/models"');
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
