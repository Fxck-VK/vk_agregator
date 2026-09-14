import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(),
  useSearchParams: vi.fn(),
}));

vi.mock("@/features/models/generation-model-catalog", () => ({
  loadGenerationModelCatalog: vi.fn(),
}));

import { chatModelForSelector } from "@/features/models/chat-model-selector";
import {
  loadGenerationModelCatalog,
  type GenerationModelCatalog,
} from "@/features/models/generation-model-catalog";
import {
  parseModelCatalog,
  projectChatModelCatalog,
  projectImageModelCatalog,
} from "@/features/models/model-catalog-contract";
import { ru } from "@/i18n/ru";
import type { ChatModel, ImageModel, ImageModelList } from "@/lib/web-api/contracts";
import { makeModelCatalogFixture } from "@/test/model-catalog";
import { useRouter, useSearchParams } from "next/navigation";

import {
  useWorkspaceModelSelection,
  WorkspaceModelSelectionProvider,
} from "../WorkspaceModelSelection/WorkspaceModelSelection";
import { WorkspaceModelSelector } from "./WorkspaceModelSelector";

const catalogue: ImageModelList = {
  items: [
    {
      categories: ["popular", "images"],
      id: "nano-banana-2",
      name: "Nano Banana 2",
      description: "Быстрая генерация и редактирование изображений для повседневных задач",
      quality_options: ["1K", "2K"],
      price_by_quality: { "1K": 16, "2K": 60 },
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 4,
    },
    {
      categories: ["popular", "images"],
      id: "gpt-image-2",
      name: "GPT Image 2",
      description: "Точное создание изображений по описанию с хорошей передачей текста",
      quality_options: ["1K"],
      price_by_quality: { "1K": 51 },
      default_quality: "1K",
      supports_reference_image: false,
      max_reference_images: 0,
    },
    {
      categories: ["popular", "images"],
      id: "nano-banana-pro",
      name: "Nano Banana Pro",
      description: "Детализированные изображения для сложных творческих и рабочих задач",
      quality_options: ["1K", "2K"],
      price_by_quality: { "1K": 50, "2K": 80 },
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 4,
    },
  ],
};

const textModels: ChatModel[] = [
  { categories: ["popular", "text", "free", "study-work"], id: "chatgpt", name: "NeiroHub Chat", estimate_credits: 0 },
  { categories: ["popular", "text", "study-work"], id: "gpt_5_5", name: "GPT-5.5", estimate_credits: 20, max_output_tokens: 2048 },
];

function makeGenerationCatalog({
  images = catalogue.items,
  text = textModels,
  categoryErrors = {},
}: {
  images?: ImageModel[];
  text?: ChatModel[];
  categoryErrors?: GenerationModelCatalog["categoryErrors"];
} = {}): GenerationModelCatalog {
  const publicCatalog = parseModelCatalog(makeModelCatalogFixture({
    defaultModelId: images[0]?.id ?? text[0]?.id,
    images,
    text,
  }));
  const imageItems = projectImageModelCatalog(publicCatalog).items.map((model) => ({
    ...model,
    category: "images" as const,
  }));
  const textItems = text.length === 0 ? [] : projectChatModelCatalog(publicCatalog).items.map((model) => ({
    ...chatModelForSelector(model),
    category: "text" as const,
  }));
  return {
    items: [...imageItems, ...textItems],
    default_model_id: publicCatalog.default_model_id,
    categoryErrors,
  };
}

function SelectionControl() {
  const selection = useWorkspaceModelSelection();

  return (
    <button onClick={() => selection?.setSelectedModelId("gpt-image-2")} type="button">
      Select GPT outside
    </button>
  );
}

function categoryOptions(name = "Популярные") {
  return within(screen.getByRole("region", { name }));
}

describe("WorkspaceModelSelector", () => {
  const push = vi.fn();

  beforeEach(() => {
    push.mockReset();
    vi.mocked(useRouter).mockReset();
    vi.mocked(useSearchParams).mockReset();
    vi.mocked(loadGenerationModelCatalog).mockReset();
    vi.mocked(useRouter).mockReturnValue({ push } as never);
    vi.mocked(useSearchParams).mockReturnValue(new URLSearchParams() as never);
    vi.mocked(loadGenerationModelCatalog).mockResolvedValue(makeGenerationCatalog());
  });

  afterEach(() => {
    cleanup();
  });

  it("loads the safe catalogue once and opens a searchable image-model list", async () => {
    render(<WorkspaceModelSelector />);

    const trigger = await screen.findByRole("button", { name: new RegExp("Nano Banana 2") });
    expect(loadGenerationModelCatalog).toHaveBeenCalledTimes(1);
    const chevron = trigger.querySelector('img[src="/assets/icons/ui/faq-arrow.svg"]');
    expect(chevron).toBeInTheDocument();
    expect(chevron).toHaveAttribute("width", "14");
    expect(chevron).toHaveAttribute("height", "8");
    expect(trigger.querySelector('img[src="/assets/icons/ui/chevron-down.svg"]')).not.toBeInTheDocument();
    expect(within(trigger).getByTestId("model-icon-fallback")).toBeInTheDocument();

    fireEvent.click(trigger);

    const dialog = screen.getByRole("dialog", { name: ru.modelSelector.dialogLabel });
    expect(dialog).toHaveAttribute("data-state", "open");
    const searchbox = screen.getByRole("searchbox", { name: ru.modelSelector.searchLabel });
    expect(searchbox).toHaveFocus();
    expect(searchbox.parentElement?.querySelector(
      'img[src="/assets/icons/ui/search.svg"]',
    )).toBeInTheDocument();
    expect(searchbox.parentElement).not.toHaveTextContent("⌕");
    expect(screen.getByRole("button", { name: ru.modelSelector.categories.images })).toBeInTheDocument();
    expect(categoryOptions().getByRole("button", { name: /Nano Banana 2/ })).not.toHaveTextContent("✦");
    expect(categoryOptions().getByRole("button", { name: /Nano Banana 2/ })).toHaveTextContent(
      "Быстрая генерация и редактирование изображений для повседневных задач",
    );
    const selectedOption = categoryOptions().getByRole("button", { name: /Nano Banana 2/ });
    const unselectedOption = categoryOptions().getByRole("button", { name: /GPT Image 2/ });
    expect(selectedOption).toHaveAttribute("aria-pressed", "true");
    expect(selectedOption.querySelector('[data-icon="check"]')).toBeNull();
    expect(unselectedOption).toHaveAttribute("aria-pressed", "false");
    expect(unselectedOption.querySelector('[data-icon="check"]')).toBeNull();
    expect(selectedOption).not.toHaveTextContent("●");
    expect(categoryOptions().getAllByTestId("model-icon-fallback")).toHaveLength(5);
    const catalogueLink = screen.getByRole("link", { name: ru.modelSelector.openCatalogue });
    expect(catalogueLink).toHaveAttribute("href", "/app/models");
  });

  it("loads text models from the chat catalogue and opens a draft with the chosen model", async () => {
    render(<WorkspaceModelSelector />);
    fireEvent.click(await screen.findByRole("button", { name: /Nano Banana 2/ }));
    fireEvent.click(screen.getByRole("button", { name: "Текст" }));
    expect(categoryOptions("Изображения").getByRole("button", { name: /Nano Banana Pro/ })).toBeInTheDocument();
    expect(categoryOptions("Текст").getByRole("button", { name: /GPT-5.5/ })).toHaveTextContent("GPT-5.5: описание из каталога");
    fireEvent.click(screen.getByRole("button", { name: "Бесплатные" }));
    expect(categoryOptions("Бесплатные").queryByRole("button", { name: /GPT-5.5/ })).toBeNull();
    expect(categoryOptions("Бесплатные").getByRole("button", { name: /NeiroHub Chat/ })).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Текст" }));
    fireEvent.click(categoryOptions("Текст").getByRole("button", { name: /GPT-5.5/ }));
    expect(push).toHaveBeenCalledExactlyOnceWith("/app/chats?model=gpt_5_5");
    expect(loadGenerationModelCatalog).toHaveBeenCalledTimes(1);
  });

  it("keeps the image catalogue available when the chat catalogue fails", async () => {
    vi.mocked(loadGenerationModelCatalog).mockResolvedValueOnce(makeGenerationCatalog({
      text: [],
      categoryErrors: {
        text: ru.modelsCatalog.loadFailure,
        free: ru.modelsCatalog.loadFailure,
        "study-work": ru.modelsCatalog.loadFailure,
      },
    }));
    render(<WorkspaceModelSelector />);
    fireEvent.click(await screen.findByRole("button", { name: /Nano Banana 2/ }));
    fireEvent.click(screen.getByRole("button", { name: "Текст" }));
    expect(categoryOptions("Текст").getByRole("status")).toHaveTextContent(/Не удалось загрузить/);
    fireEvent.click(screen.getByRole("button", { name: "Изображения" }));
    expect(categoryOptions("Изображения").getByRole("button", { name: /Nano Banana Pro/ })).toBeVisible();
  });

  it("shows the available image selection and hides empty categories", async () => {
    render(<WorkspaceModelSelector />);
    fireEvent.click(await screen.findByRole("button", { name: /Nano Banana 2/ }));
    fireEvent.click(screen.getByRole("button", { name: ru.modelSelector.categories.images }));
    const dialog = screen.getByRole("dialog");
    expect(categoryOptions("Изображения").getAllByRole("listitem")).toHaveLength(3);
    for (const model of catalogue.items) {
      expect(categoryOptions("Изображения").getAllByRole("button", { name: new RegExp(model.name) })).toHaveLength(1);
    }
    expect(within(dialog).queryByRole("button", { name: "Видео и аудио" })).not.toBeInTheDocument();
    expect(within(dialog).queryByRole("region", { name: "Видео и аудио" })).not.toBeInTheDocument();
  });

  it("filters locally, selects a model, closes, and navigates without a reload", async () => {
    render(<WorkspaceModelSelector />);
    fireEvent.click(await screen.findByRole("button", { name: new RegExp("Nano Banana 2") }));

    fireEvent.change(screen.getByRole("searchbox", { name: ru.modelSelector.searchLabel }), {
      target: { value: "GPT" },
    });

    const dialog = screen.getByRole("dialog", { name: ru.modelSelector.dialogLabel });
    expect(within(dialog).queryByRole("button", { name: /Nano Banana 2/ })).toBeNull();
    const option = categoryOptions("Изображения").getByRole("button", { name: /GPT Image 2/ });
    expect(option).not.toHaveTextContent("✦");
    fireEvent.click(option);

    expect(push).toHaveBeenCalledExactlyOnceWith("/app/chats?model=gpt-image-2");
    const closingDialog = document.getElementById("workspace-model-selector-dialog");
    expect(closingDialog).toHaveAttribute("data-state", "closing");
    expect(closingDialog).toHaveAttribute("aria-hidden", "true");
    fireEvent.animationEnd(closingDialog as HTMLElement, {
      animationName: "workspaceModelSelectorClose",
    });
    expect(document.getElementById("workspace-model-selector-dialog")).toBeNull();
    expect(screen.getByRole("button", { name: new RegExp("GPT Image 2") })).toHaveFocus();
  });

  it("reflects a model from a direct generator URL", async () => {
    vi.mocked(useSearchParams).mockReturnValue(new URLSearchParams("model=gpt-image-2") as never);

    render(<WorkspaceModelSelector />);

    expect(await screen.findByRole("button", { name: new RegExp("GPT Image 2") })).toBeInTheDocument();
  });

  it("reflects a model changed by another workspace editor", async () => {
    render(
      <WorkspaceModelSelectionProvider>
        <WorkspaceModelSelector />
        <SelectionControl />
      </WorkspaceModelSelectionProvider>,
    );
    await screen.findByRole("button", { name: new RegExp("Nano Banana 2") });

    fireEvent.click(screen.getByRole("button", { name: "Select GPT outside" }));

    expect(screen.getByRole("button", { name: new RegExp("GPT Image 2") })).toBeInTheDocument();
  });

  it("finishes the animated close after Escape or an outside press", async () => {
    render(
      <div>
        <WorkspaceModelSelector />
        <button type="button">Outside</button>
      </div>,
    );
    const trigger = await screen.findByRole("button", { name: new RegExp("Nano Banana 2") });

    fireEvent.click(trigger);
    fireEvent.keyDown(document, { key: "Escape" });
    const escapeClosingDialog = document.getElementById("workspace-model-selector-dialog");
    expect(escapeClosingDialog).toHaveAttribute("data-state", "closing");
    expect(escapeClosingDialog).toHaveAttribute("aria-hidden", "true");
    expect(trigger).toHaveFocus();
    fireEvent.animationEnd(escapeClosingDialog as HTMLElement, {
      animationName: "workspaceModelSelectorClose",
    });
    expect(document.getElementById("workspace-model-selector-dialog")).toBeNull();

    fireEvent.click(trigger);
    fireEvent.pointerDown(screen.getByRole("button", { name: "Outside" }));
    const outsideClosingDialog = document.getElementById("workspace-model-selector-dialog");
    expect(outsideClosingDialog).toHaveAttribute("data-state", "closing");
    fireEvent.animationEnd(outsideClosingDialog as HTMLElement, {
      animationName: "workspaceModelSelectorClose",
    });
    await waitFor(() => {
      expect(document.getElementById("workspace-model-selector-dialog")).toBeNull();
    });
  });

  it("cancels an exit animation when the trigger reopens the selector", async () => {
    render(<WorkspaceModelSelector />);
    const trigger = await screen.findByRole("button", { name: new RegExp("Nano Banana 2") });

    fireEvent.click(trigger);
    fireEvent.click(trigger);
    const closingDialog = document.getElementById("workspace-model-selector-dialog");
    expect(closingDialog).toHaveAttribute("data-state", "closing");

    fireEvent.click(trigger);
    expect(closingDialog).toHaveAttribute("data-state", "open");
    fireEvent.animationEnd(closingDialog as HTMLElement, {
      animationName: "workspaceModelSelectorClose",
    });
    expect(document.getElementById("workspace-model-selector-dialog")).toBe(closingDialog);
  });

  it("shows a truthful failure state and does not invent a model", async () => {
    vi.mocked(loadGenerationModelCatalog).mockResolvedValueOnce({
      items: [],
      default_model_id: "",
      categoryErrors: {
        popular: ru.modelsCatalog.loadFailure,
        images: ru.modelsCatalog.loadFailure,
        text: ru.modelsCatalog.loadFailure,
        "video-audio": ru.modelsCatalog.loadFailure,
        free: ru.modelsCatalog.loadFailure,
        "study-work": ru.modelsCatalog.loadFailure,
      },
    });

    render(<WorkspaceModelSelector />);

    expect(await screen.findByRole("button", { name: ru.modelSelector.unavailable })).toBeDisabled();
    expect(screen.queryByText("Nano Banana 2")).toBeNull();
  });
});
