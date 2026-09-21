import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

import { ModelSelector, type ModelSelectorModel } from "./ModelSelector";

afterEach(cleanup);

const models: ModelSelectorModel[] = [
  ...Array.from({ length: 7 }, (_, index) => ({
    categories: ["popular", "images"],
    category: "images" as const,
    id: `image-${index}`,
    name: `Image ${index}`,
  })),
  { categories: ["popular", "text", "free", "study-work"], id: "chat", name: "Chat", category: "text", isFree: true },
  { categories: ["popular", "video-audio"], id: "video", name: "Video", category: "video" },
];

function openSelector(props: Partial<Parameters<typeof ModelSelector>[0]> = {}) {
  render(<ModelSelector models={models} onSelect={vi.fn()} selectedModelId="image-0" {...props} />);
  fireEvent.click(screen.getByRole("button", { name: /Выбрана нейросеть/ }));
  return screen.getByRole("dialog");
}

it("keeps the composer panel height fixed across categories and search results", () => {
  const dialog = openSelector({ variant: "composer", renderInPortal: true, descriptionMode: "tooltip" });
  const height = dialog.style.blockSize;
  expect(Number.parseFloat(height)).toBeGreaterThan(0);
  fireEvent.click(within(screen.getByRole("toolbar")).getByRole("button", { name: "Текст" }));
  expect(dialog.style.blockSize).toBe(height);
  fireEvent.change(screen.getByRole("searchbox"), { target: { value: "chat" } });
  expect(dialog.style.blockSize).toBe(height);
  fireEvent.change(screen.getByRole("searchbox"), { target: { value: "missing" } });
  expect(screen.getByRole("status")).toHaveTextContent("Подходящих нейросетей не найдено.");
  expect(dialog.style.blockSize).toBe(height);
});

it("keeps video and audio in separate selectable sections", () => {
  const video = { categories: ["video-audio"], id: "video", name: "Video model", category: "video" as const };
  const audio = { categories: ["video-audio"], id: "audio", name: "Audio model", category: "audio" as const };
  const onSelect = vi.fn();
  openSelector({ models: [video, audio], selectedModelId: video.id, onSelect });

  expect(within(screen.getByRole("region", { name: "Видео" })).getByRole("button", { name: /Video model/ })).toBeInTheDocument();
  const audioSection = screen.getByRole("region", { name: "Аудио" });
  expect(within(audioSection).queryByRole("button", { name: /Video model/ })).not.toBeInTheDocument();
  fireEvent.click(within(screen.getByRole("toolbar")).getByRole("button", { name: "Аудио" }));
  expect(screen.getAllByRole("heading")[0]).toHaveTextContent("Аудио");
  fireEvent.click(within(audioSection).getByRole("button", { name: /Audio model/ }));
  expect(onSelect).toHaveBeenCalledExactlyOnceWith(audio);
});

it("shows a continuous feed capped at five models per category and promotes a section without removing others", () => {
  const onSelect = vi.fn();
  const dialog = openSelector({ onSelect });
  const headings = () => within(dialog).getAllByRole("heading").map((heading) => heading.textContent);
  expect(headings()).toEqual(["Популярные", "Изображения", "Текст", "Видео", "Бесплатные", "Учёба и работа"]);
  expect(within(screen.getByRole("region", { name: "Изображения" })).getAllByRole("listitem")).toHaveLength(5);
  expect(within(dialog).queryByRole("button", { name: /Image 5/ })).not.toBeInTheDocument();

  const feed = within(dialog).getByRole("region", { name: "Подборки нейросетей" });
  feed.scrollTop = 200;
  fireEvent.click(within(screen.getByRole("toolbar")).getByRole("button", { name: "Текст" }));
  expect(headings()).toEqual(["Текст", "Популярные", "Изображения", "Видео", "Бесплатные", "Учёба и работа"]);
  expect(feed.scrollTop).toBe(0);
  expect(onSelect).not.toHaveBeenCalled();
  fireEvent.click(within(screen.getByRole("region", { name: "Видео" })).getByRole("button", { name: /Video/ }));
  expect(onSelect).toHaveBeenCalledExactlyOnceWith(models.at(-1));
});

it("accepts ordered model ID selections, removes duplicates and unavailable IDs, and hides empty categories", () => {
  const dialog = openSelector({ categoryModelIds: {
    popular: [], images: ["missing", "chat", "image-6", "image-6", "image-5", "image-4", "image-3", "image-2", "image-1"],
    text: [], free: [], "study-work": [], video: [], audio: [],
  } });
  expect(within(dialog).getAllByRole("heading").map((heading) => heading.textContent)).toEqual(["Изображения"]);
  expect(within(dialog).getAllByRole("listitem").map((item) => item.querySelector("button")?.textContent)).toEqual(
    [6, 5, 4, 3, 2].map((index) => expect.stringContaining(`Image ${index}`)),
  );
  expect(within(screen.getByRole("toolbar")).getAllByRole("button")).toHaveLength(1);
  expect(within(screen.getByRole("toolbar")).getByRole("button")).toHaveAttribute("aria-pressed", "true");
});

it("uses server category memberships as authoritative when they are present", () => {
  const serverModels: ModelSelectorModel[] = [
    {
      categories: ["popular", "text"],
      category: "images",
      id: "server-text",
      isFree: true,
      name: "Server Text",
    },
    {
      categories: ["images", "free"],
      category: "text",
      id: "server-image",
      name: "Server Image",
    },
  ];
  const dialog = openSelector({ models: serverModels, selectedModelId: "server-text" });

  expect(within(dialog).getAllByRole("heading").map((heading) => heading.textContent)).toEqual([
    "Популярные",
    "Изображения",
    "Текст",
    "Бесплатные",
  ]);
  expect(within(screen.getByRole("region", { name: "Текст" })).getByRole("button", { name: /Server Text/ })).toBeInTheDocument();
  expect(within(screen.getByRole("region", { name: "Изображения" })).queryByRole("button", { name: /Server Text/ })).not.toBeInTheDocument();
  expect(within(screen.getByRole("region", { name: "Бесплатные" })).getByRole("button", { name: /Server Image/ })).toBeInTheDocument();
  expect(within(dialog).queryByRole("region", { name: "Учёба и работа" })).not.toBeInTheDocument();
});

it("does not infer category membership when server categories are absent", () => {
  const uncategorizedModels: ModelSelectorModel[] = [
    {
      category: "images",
      id: "uncategorized-image",
      isFree: true,
      name: "Uncategorized Image",
    },
  ];
  openSelector({ models: uncategorizedModels, selectedModelId: "uncategorized-image" });

  expect(screen.queryByRole("toolbar")).not.toBeInTheDocument();
  expect(screen.getByRole("status")).toHaveTextContent("Подходящих нейросетей не найдено.");
});

it("searches all curated sections, keeps category controls and never pulls uncurated models into results", () => {
  const dialog = openSelector();
  fireEvent.click(within(screen.getByRole("toolbar")).getByRole("button", { name: "Изображения" }));
  fireEvent.change(screen.getByRole("searchbox"), { target: { value: "chat" } });
  expect(within(dialog).getAllByRole("heading").map((heading) => heading.textContent)).toEqual(["Текст", "Бесплатные", "Учёба и работа"]);
  expect(within(screen.getByRole("toolbar")).getAllByRole("button")).toHaveLength(6);
  fireEvent.change(screen.getByRole("searchbox"), { target: { value: "image-6" } });
  expect(screen.getByRole("status")).toHaveTextContent("Подходящих нейросетей не найдено.");
  expect(within(dialog).queryAllByRole("listitem")).toHaveLength(0);
  fireEvent.change(screen.getByRole("searchbox"), { target: { value: "" } });
  expect(within(dialog).getAllByRole("heading")[0]).toHaveTextContent("Изображения");
});

it("keeps a failed category visible alongside working sections but hides deliberately empty selections", () => {
  const dialog = openSelector({
    models: models.filter((model) => model.category !== "text"),
    categoryErrors: { text: "Не удалось загрузить текстовые модели", free: "Не удалось загрузить текстовые модели" },
    categoryModelIds: { free: [] },
  });
  expect(within(screen.getByRole("region", { name: "Текст" })).getByRole("status")).toHaveTextContent("Не удалось загрузить");
  expect(within(dialog).queryByRole("region", { name: "Бесплатные" })).not.toBeInTheDocument();
  expect(within(dialog).getByRole("region", { name: "Изображения" })).toBeInTheDocument();
});

it("shows a single tooltip even when the same model occurs in multiple sections", () => {
  openSelector({ descriptionMode: "tooltip" });
  const popularOption = within(screen.getByRole("region", { name: "Популярные" })).getByRole("button", { name: "Image 0" });
  const imageOption = within(screen.getByRole("region", { name: "Изображения" })).getByRole("button", { name: "Image 0" });
  fireEvent.focus(popularOption);
  fireEvent.pointerEnter(imageOption);
  expect(screen.getAllByRole("tooltip")).toHaveLength(1);
});
