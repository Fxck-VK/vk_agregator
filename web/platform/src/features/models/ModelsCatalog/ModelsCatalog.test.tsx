import type { ReactNode } from "react";

import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    prefetch,
    ...props
  }: {
    children: ReactNode;
    href: string;
    prefetch?: boolean;
  }) => (
    <a data-next-link="true" data-prefetch={String(prefetch)} href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("../generation-model-catalog", () => ({
  loadGenerationModelCatalog: vi.fn(),
}));

import { ru } from "@/i18n/ru";

import { loadGenerationModelCatalog } from "../generation-model-catalog";

import { ModelsCatalog } from "./ModelsCatalog";

const modelsResponse = {
  default_model_id: "text-model-1",
  categoryErrors: {},
  items: [
    {
      id: "nano banana/2&preview",
      name: "Nano Banana",
      description: "Server supplied image description",
      category: "images",
      categories: ["popular", "images"],
      quality_options: ["1K", "2K"],
      price_by_quality: { "1K": 16, "2K": 60 },
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 1,
    },
    {
      id: "other-model",
      name: "Other Model",
      description: "Server supplied fallback image description",
      category: "images",
      categories: ["popular", "images"],
      quality_options: ["4K"],
      default_quality: "4K",
      supports_reference_image: false,
      max_reference_images: 0,
    },
    {
      id: "category-authoritative",
      name: "Text Shaped By Server Category",
      description: "Server category wins over local kind",
      category: "images",
      categories: ["text"],
    },
    ...Array.from({ length: 6 }, (_, index) => ({
      id: `text-model-${index + 1}`,
      name: `Text Model ${index + 1}`,
      description: `Text description ${index + 1}`,
      category: "text",
      categories: ["text", index === 0 ? "study-work" : "popular"].filter(Boolean),
    })),
    {
      id: "video-model",
      name: "Video Model",
      description: "Video description",
      category: "video",
      categories: ["video-audio"],
    },
  ],
} as Awaited<ReturnType<typeof loadGenerationModelCatalog>>;

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("ModelsCatalog", () => {
  it("omits the standalone NeiroHub eyebrow while keeping the catalogue title", () => {
    vi.mocked(loadGenerationModelCatalog).mockReturnValue(new Promise(() => {}));
    render(<ModelsCatalog />);

    expect(screen.queryByText("NeiroHub", { exact: true, selector: "header > p" })).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: ru.modelsCatalog.title })).toBeInTheDocument();
  });

  it("loads generation catalog data, renders server descriptions, and links cards to new chat", async () => {
    vi.mocked(loadGenerationModelCatalog).mockResolvedValue(modelsResponse);
    render(<ModelsCatalog />);

    expect(loadGenerationModelCatalog).toHaveBeenCalledTimes(1);
    expect(await screen.findByRole("link", { name: `${ru.modelsCatalog.openGeneratorLabel}: Nano Banana` })).toHaveAttribute(
      "href",
      "/app/chats?model=nano%20banana%2F2%26preview",
    );
    expect(screen.getByRole("link", { name: `${ru.modelsCatalog.openGeneratorLabel}: Nano Banana` })).toHaveAttribute(
      "data-next-link",
      "true",
    );
    expect(screen.getByRole("link", { name: `${ru.modelsCatalog.openGeneratorLabel}: Nano Banana` })).toHaveAttribute(
      "data-prefetch",
      "false",
    );
    const nanoCard = screen.getByText("Nano Banana").closest("article")!;
    const otherCard = screen.getByText("Other Model").closest("article")!;
    expect(within(nanoCard).getByText("Server supplied image description")).toBeInTheDocument();
    expect(within(otherCard).getByText("Server supplied fallback image description")).toBeInTheDocument();
    expect(within(nanoCard).queryByRole("list", { name: ru.modelsCatalog.qualityFilterLabel })).toBeNull();
    expect(within(nanoCard).queryByText(ru.modelsCatalog.referenceSupportedLabel)).toBeNull();
    expect(within(nanoCard).getByLabelText("16 звёзд")).toBeInTheDocument();

    fireEvent.change(screen.getByRole("searchbox", { name: ru.modelsCatalog.searchLabel }), {
      target: { value: "banana" },
    });
    expect(screen.queryByText("Other Model")).not.toBeInTheDocument();
  });

  it("does not add local placeholder cards to the functional catalog", async () => {
    vi.mocked(loadGenerationModelCatalog).mockResolvedValue(modelsResponse);
    render(<ModelsCatalog />);

    expect(await screen.findByRole("heading", { name: "Nano Banana" })).toBeInTheDocument();
    for (const name of ["Recraft V3", "Ideogram 3", "Stable Diffusion 3.5", "Leonardo Phoenix"]) {
      expect(screen.queryByRole("heading", { name })).not.toBeInTheDocument();
    }

    fireEvent.change(screen.getByRole("searchbox", { name: ru.modelsCatalog.searchLabel }), {
      target: { value: "recraft" },
    });

    expect(screen.queryByRole("heading", { name: "Recraft V3" })).toBeNull();
    expect(screen.getByText(ru.modelsCatalog.empty)).toBeInTheDocument();
  });

  it("asks the shared loader on every catalogue mount", async () => {
    vi.mocked(loadGenerationModelCatalog).mockResolvedValue(modelsResponse);
    const firstMount = render(<ModelsCatalog />);

    await screen.findByText("Nano Banana");
    firstMount.unmount();
    render(<ModelsCatalog />);

    await screen.findByText("Nano Banana");
    expect(loadGenerationModelCatalog).toHaveBeenCalledTimes(2);
  });

  it("shows a loading status while the catalog request is pending", () => {
    vi.mocked(loadGenerationModelCatalog).mockReturnValue(new Promise(() => {}));
    render(<ModelsCatalog />);

    expect(screen.getByRole("status")).toHaveTextContent(ru.modelsCatalog.loading);
  });

  it.each([
    ["a rejected loader", () => Promise.reject(new Error("untrusted backend detail"))],
  ])("shows a neutral alert after %s", async (_caseName, load) => {
    vi.mocked(loadGenerationModelCatalog).mockImplementationOnce(load);
    render(<ModelsCatalog />);

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.modelsCatalog.loadFailure);
    expect(screen.queryByText("untrusted backend detail")).not.toBeInTheDocument();
  });

  it("distinguishes a valid empty catalog from a load failure", async () => {
    vi.mocked(loadGenerationModelCatalog).mockResolvedValue({ default_model_id: "", categoryErrors: {}, items: [] });
    render(<ModelsCatalog />);

    expect(await screen.findByText(ru.modelsCatalog.empty)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("filters each full catalogue tab by server categories without a five-item cap", async () => {
    vi.mocked(loadGenerationModelCatalog).mockResolvedValue(modelsResponse);
    render(<ModelsCatalog />);

    await screen.findByText("Nano Banana");
    expect(screen.getByRole("heading", { name: "Популярные" })).toBeInTheDocument();
    expect(screen.getByRole("tabpanel", { name: "Популярные" })).toHaveAttribute("id", "models-catalog-panel");
    for (const category of ["Популярные", "Изображения", "Текст", "Видео и аудио", "Бесплатные", "Учёба и работа"]) {
      expect(screen.getByRole("tab", { name: category })).toBeInTheDocument();
    }
    expect(screen.getByText("Nano Banana")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Изображения" }));
    expect(screen.getByText("Nano Banana")).toBeInTheDocument();
    expect(screen.getByText("Other Model")).toBeInTheDocument();
    expect(screen.queryByText("Text Shaped By Server Category")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Текст" }));

    expect(screen.getByRole("tabpanel", { name: "Текст" })).toHaveAttribute(
      "aria-labelledby",
      "model-category-tab-text",
    );
    expect(screen.getByText("Text Shaped By Server Category")).toBeInTheDocument();
    for (let index = 1; index <= 6; index += 1) {
      expect(screen.getByText(`Text Model ${index}`)).toBeInTheDocument();
    }
    expect(screen.queryByText("Nano Banana")).not.toBeInTheDocument();
  });
});
