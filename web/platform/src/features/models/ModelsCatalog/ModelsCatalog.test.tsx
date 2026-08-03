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

vi.mock("../image-model-catalog-cache", () => ({
  loadImageModelCatalog: vi.fn(),
}));

import { ru } from "@/i18n/ru";

import { loadImageModelCatalog } from "../image-model-catalog-cache";

import { ModelsCatalog } from "./ModelsCatalog";

const modelsResponse = {
  items: [
    {
      id: "nano banana/2&preview",
      name: "Nano Banana",
      quality_options: ["1K", "2K"],
      price_by_quality: { "1K": 16, "2K": 60 },
      default_quality: "1K",
      supports_reference_image: true,
      max_reference_images: 1,
    },
    {
      id: "other-model",
      name: "Other Model",
      quality_options: ["4K"],
      default_quality: "4K",
      supports_reference_image: false,
      max_reference_images: 0,
    },
    {
      id: "reference-2k",
      name: "Reference 2K",
      quality_options: ["2K"],
      default_quality: "2K",
      supports_reference_image: true,
      max_reference_images: 2,
    },
  ],
};

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("ModelsCatalog", () => {
  it("loads catalog data, exposes truthful DTO card facts, and links to the selected generator", async () => {
    vi.mocked(loadImageModelCatalog).mockResolvedValue(modelsResponse);
    render(<ModelsCatalog />);

    expect(loadImageModelCatalog).toHaveBeenCalledTimes(1);
    expect(await screen.findByRole("link", { name: `${ru.modelsCatalog.openGeneratorLabel}: Nano Banana` })).toHaveAttribute(
      "href",
      "/app/image?model=nano%20banana%2F2%26preview",
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
    expect(within(nanoCard).getByText(ru.modelsCatalog.imageTypeLabel)).toBeInTheDocument();
    expect(within(nanoCard).getByText("1K")).toBeInTheDocument();
    expect(within(nanoCard).getByText("2K")).toBeInTheDocument();
    expect(within(nanoCard).getByText(ru.modelsCatalog.referenceSupportedLabel)).toBeInTheDocument();
    expect(within(otherCard).getByText(ru.modelsCatalog.referenceUnsupportedLabel)).toBeInTheDocument();
    expect(within(nanoCard).getByText("От 16 ★")).toBeInTheDocument();

    fireEvent.change(screen.getByRole("searchbox", { name: ru.modelsCatalog.searchLabel }), {
      target: { value: "banana" },
    });
    expect(screen.queryByText("Other Model")).not.toBeInTheDocument();
  });

  it("asks the shared loader on every catalogue mount", async () => {
    vi.mocked(loadImageModelCatalog).mockResolvedValue(modelsResponse);
    const firstMount = render(<ModelsCatalog />);

    await screen.findByText("Nano Banana");
    firstMount.unmount();
    render(<ModelsCatalog />);

    await screen.findByText("Nano Banana");
    expect(loadImageModelCatalog).toHaveBeenCalledTimes(2);
  });

  it("shows a loading status while the catalog request is pending", () => {
    vi.mocked(loadImageModelCatalog).mockReturnValue(new Promise(() => {}));
    render(<ModelsCatalog />);

    expect(screen.getByRole("status")).toHaveTextContent(ru.modelsCatalog.loading);
  });

  it.each([
    ["a rejected loader", () => Promise.reject(new Error("untrusted backend detail"))],
  ])("shows a neutral alert after %s", async (_caseName, load) => {
    vi.mocked(loadImageModelCatalog).mockImplementationOnce(load);
    render(<ModelsCatalog />);

    expect(await screen.findByRole("alert")).toHaveTextContent(ru.modelsCatalog.loadFailure);
    expect(screen.queryByText("untrusted backend detail")).not.toBeInTheDocument();
  });

  it("distinguishes a valid empty catalog from a load failure", async () => {
    vi.mocked(loadImageModelCatalog).mockResolvedValue({ items: [] });
    render(<ModelsCatalog />);

    expect(await screen.findByText(ru.modelsCatalog.empty)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows the popular image catalogue and switches future categories to their planned state", async () => {
    vi.mocked(loadImageModelCatalog).mockResolvedValue(modelsResponse);
    render(<ModelsCatalog />);

    await screen.findByText("Nano Banana");
    expect(screen.getByRole("heading", { name: "Популярные" })).toBeInTheDocument();
    expect(screen.getByRole("tabpanel", { name: "Популярные" })).toHaveAttribute("id", "models-catalog-panel");
    for (const category of ["Популярные", "Изображения", "Текст", "Видео и аудио", "Бесплатные", "Учёба и работа"]) {
      expect(screen.getByRole("tab", { name: category })).toBeInTheDocument();
    }
    expect(screen.getByText("Nano Banana")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Текст" }));

    expect(await screen.findByText("Категория «Текст» появится позже.")).toBeInTheDocument();
    expect(screen.getByRole("tabpanel", { name: "Текст" })).toHaveAttribute(
      "aria-labelledby",
      "model-category-tab-text",
    );
    expect(screen.queryByText("Nano Banana")).not.toBeInTheDocument();
  });
});
