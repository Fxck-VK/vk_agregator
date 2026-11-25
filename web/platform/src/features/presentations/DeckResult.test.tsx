import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { DeckResult } from "./DeckResult";
import { defaultDeckAppearance, type DeckAppearance } from "./deck-appearance";
import { emptyBrief, type DeckSlide } from "./deck-content";
import type { SlideImageFrame } from "./slide-image-frame";

vi.mock("@/features/landing-variants/third/HomeMedia", () => ({
  HomePicture: () => <span />,
}));

vi.mock("./SlideImageGeneration", () => ({
  SlideImageGeneration: ({ frame, onImage, onClose }: { frame: SlideImageFrame; onImage: (file: Blob) => Promise<void>; onClose: () => void }) => (
    <section role="dialog" aria-label="Изображение для слайда">
      <output>{JSON.stringify(frame)}</output>
      <button type="button" onClick={() => void onImage(new Blob(["generated"], { type: "image/jpeg" })).then(onClose)}>Добавить готовое фото</button>
    </section>
  ),
}));

const brief = { ...emptyBrief, topic: "Поездка по Италии" };

function Editor({ initialAppearance = defaultDeckAppearance, canGenerateImage = false }: { initialAppearance?: DeckAppearance; canGenerateImage?: boolean }) {
  const [slides, setSlides] = useState<DeckSlide[]>([
    { title: "Италия", points: ["Обложка"] },
    { title: "Встреча", points: ["Тезис"] },
  ]);
  const [appearance, setAppearance] = useState(initialAppearance);
  return <DeckResult slides={slides} brief={brief} appearance={appearance} canGenerateImage={canGenerateImage} onAppearanceChange={setAppearance} onChange={setSlides} onNew={() => {}} />;
}

describe("presentation slide editor", () => {
  it("previews valid edits and restores the saved slide when cancelled", () => {
    render(<Editor />);
    fireEvent.click(screen.getByRole("button", { name: "Изменить слайд" }));
    const title = screen.getByRole("textbox", { name: "Заголовок слайда" });
    fireEvent.change(title, { target: { value: "Маршрут по Италии" } });
    expect(screen.getByRole("heading", { name: "Маршрут по Италии" })).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Отмена" }));
    expect(screen.getByRole("heading", { name: "Италия" })).toBeVisible();
    expect(screen.queryByRole("heading", { name: "Маршрут по Италии" })).not.toBeInTheDocument();
  });

  it("replaces the cover with a local picture and restores the standard cover", async () => {
    const view = render(<Editor />);
    const drawImage = vi.fn();
    vi.stubGlobal("Image", class {
      naturalWidth = 650;
      naturalHeight = 778;
      decode = async () => {};
    });
    vi.stubGlobal("FileReader", class {
      result = "data:image/jpeg;base64,cHJldmlldw==";
      onload?: () => void;
      readAsDataURL() { this.onload?.(); }
    });
    vi.stubGlobal("URL", class extends URL {
      static createObjectURL = vi.fn(() => "blob:epifera-cover");
      static revokeObjectURL = vi.fn();
    });
    const canvas = vi.spyOn(HTMLCanvasElement.prototype, "getContext")
      .mockReturnValue({ drawImage } as unknown as CanvasRenderingContext2D);
    const blob = vi.spyOn(HTMLCanvasElement.prototype, "toBlob")
      .mockImplementation(callback => callback(new Blob(["preview"], { type: "image/jpeg" })));
    try {
      const input = view.container.querySelector('input[type="file"]')!;
      fireEvent.change(input, { target: { files: [new File(["picture"], "cover.png", { type: "image/png" })] } });
      await waitFor(() => expect(view.container.querySelector('img[src="data:image/jpeg;base64,cHJldmlldw=="]')).toBeInTheDocument());
      expect(drawImage).toHaveBeenCalledOnce();
      fireEvent.click(screen.getByRole("button", { name: "Вернуть стандартную обложку" }));
      expect(view.container.querySelector('img[src="data:image/jpeg;base64,cHJldmlldw=="]')).not.toBeInTheDocument();
    } finally {
      view.unmount();
      canvas.mockRestore();
      blob.mockRestore();
      vi.unstubAllGlobals();
    }
  });

  it("keeps an uploaded image with the selected slide through text editing and reordering", async () => {
    const view = render(<Editor />);
    const canvas = vi.spyOn(HTMLCanvasElement.prototype, "getContext")
      .mockReturnValue({ drawImage: vi.fn() } as unknown as CanvasRenderingContext2D);
    const blob = vi.spyOn(HTMLCanvasElement.prototype, "toBlob")
      .mockImplementation(callback => callback(new Blob(["preview"], { type: "image/jpeg" })));
    vi.stubGlobal("Image", class {
      naturalWidth = 650;
      naturalHeight = 778;
      decode = async () => {};
    });
    vi.stubGlobal("URL", class extends URL {
      static createObjectURL = vi.fn(() => "blob:epifera-slide");
      static revokeObjectURL = vi.fn();
    });
    try {
      fireEvent.click(screen.getByRole("button", { name: "Следующий слайд" }));
      expect(screen.getByRole("button", { name: "Загрузить фото" })).toBeVisible();
      const input = view.container.querySelector('input[type="file"]')!;
      fireEvent.change(input, { target: { files: [new File(["picture"], "slide.png", { type: "image/png" })] } });
      const attached = () => view.container.querySelector('article img[src="data:image/jpeg;base64,cHJldmlldw=="]');
      await waitFor(() => expect(attached()).toBeInTheDocument());
      fireEvent.click(screen.getByRole("button", { name: "Изменить слайд" }));
      fireEvent.change(screen.getByRole("textbox", { name: "Заголовок слайда" }), { target: { value: "Новый заголовок" } });
      fireEvent.click(screen.getByRole("button", { name: "Сохранить слайд" }));
      expect(attached()?.closest("article")).toHaveTextContent("Новый заголовок");
      fireEvent.click(screen.getByRole("button", { name: "Переместить раньше" }));
      expect(attached()?.closest("article")).toHaveAttribute("data-cover", "true");
      fireEvent.click(screen.getByRole("button", { name: "Следующий слайд" }));
      expect(attached()).not.toBeInTheDocument();
    } finally {
      view.unmount();
      canvas.mockRestore();
      blob.mockRestore();
      vi.unstubAllGlobals();
    }
  });

  it("commits the chosen image frame only on insertion and uses it in the editor, print and show", async () => {
    const canvas = vi.spyOn(HTMLCanvasElement.prototype, "getContext")
      .mockReturnValue({ drawImage: vi.fn() } as unknown as CanvasRenderingContext2D);
    const dimensions: number[][] = [];
    const blob = vi.spyOn(HTMLCanvasElement.prototype, "toBlob").mockImplementation(function (this: HTMLCanvasElement, callback) {
      dimensions.push([this.width, this.height]);
      callback(new Blob([`photo-${dimensions.length}`], { type: "image/jpeg" }));
    });
    vi.stubGlobal("Image", class {
      naturalWidth = 800;
      naturalHeight = 800;
      decode = async () => {};
    });
    vi.stubGlobal("URL", class extends URL {
      static createObjectURL = vi.fn(() => "blob:epifera-generated");
      static revokeObjectURL = vi.fn();
    });
    const view = render(<Editor canGenerateImage />);
    try {
      fireEvent.click(screen.getByRole("button", { name: "Следующий слайд" }));
      const generate = screen.getByRole("button", { name: "Сгенерировать изображение" });
      fireEvent.click(generate);
      const frame = screen.getByRole("group", { name: "Область изображения на слайде" });
      expect(frame).toHaveFocus();
      expect(screen.queryByRole("dialog", { name: "Изображение для слайда" })).not.toBeInTheDocument();
      fireEvent.keyDown(frame, { key: "Escape" });
      expect(generate).toHaveFocus();
      expect(view.container.querySelector("[data-custom-frame]")).not.toBeInTheDocument();

      fireEvent.click(generate);
      fireEvent.click(screen.getByText("Положение и размер"));
      for (const [name, value] of [["Слева, %", "25"], ["Ширина, %", "50"], ["Высота, %", "40"], ["Сверху, %", "10"]]) {
        const field = screen.getByRole("spinbutton", { name });
        fireEvent.change(field, { target: { value } });
        fireEvent.blur(field);
      }
      fireEvent.click(screen.getByRole("button", { name: "Выбрать модель" }));
      const dialog = await screen.findByRole("dialog", { name: "Изображение для слайда" });
      expect(within(dialog).getByRole("status")).toHaveTextContent('{"x":2500,"y":1000,"width":5000,"height":4000}');
      expect(view.container.querySelector("[data-custom-frame]")).not.toBeInTheDocument();
      fireEvent.click(within(dialog).getByRole("button", { name: "Добавить готовое фото" }));

      await waitFor(() => expect(view.container.querySelector("[data-custom-frame]")).toBeInTheDocument());
      expect(dimensions).toEqual([[650, 293]]);
      const expectedStyle = { left: "25%", top: "10%", width: "50%", height: "40%" };
      expect(document.querySelectorAll("[data-custom-frame]")).toHaveLength(2);
      document.querySelectorAll("[data-custom-frame]").forEach(node => expect(node).toHaveStyle(expectedStyle));
      const generatedSource = view.container.querySelector("[data-custom-frame] img")?.getAttribute("src");
      const upload = view.container.querySelector('input[type="file"]')!;
      fireEvent.change(upload, { target: { files: [new File(["replacement"], "replacement.png", { type: "image/png" })] } });
      await waitFor(() => expect(screen.getByRole("button", { name: "Заменить фото" })).toBeEnabled());
      expect(dimensions).toEqual([[650, 293], [650, 293]]);
      expect(view.container.querySelector("[data-custom-frame]")).toHaveStyle(expectedStyle);
      expect(view.container.querySelector("[data-custom-frame] img")?.getAttribute("src")).not.toEqual(generatedSource);
      fireEvent.click(screen.getByRole("button", { name: "Удалить фото" }));
      expect(document.querySelector("[data-custom-frame]")).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Отменить изменение" }));
      fireEvent.click(screen.getByRole("button", { name: "Начать показ" }));
      const show = screen.getByRole("dialog", { name: brief.topic });
      expect(show.querySelector("[data-custom-frame]")).toHaveStyle(expectedStyle);
    } finally {
      view.unmount();
      canvas.mockRestore();
      blob.mockRestore();
      vi.unstubAllGlobals();
    }
  });

  it("does not publish a replacement or move its frame after the editor unmounts mid-decode", async () => {
    let finishDecode!: () => void;
    const decode = vi.fn(() => new Promise<void>(resolve => { finishDecode = resolve; }));
    const revoke = vi.fn();
    const onChange = vi.fn();
    vi.stubGlobal("Image", class { naturalWidth = 800; naturalHeight = 800; decode = decode; });
    vi.stubGlobal("URL", class extends URL {
      static createObjectURL = vi.fn(() => "blob:epifera-cancelled");
      static revokeObjectURL = revoke;
    });
    const canvas = vi.spyOn(HTMLCanvasElement.prototype, "getContext")
      .mockReturnValue({ drawImage: vi.fn() } as unknown as CanvasRenderingContext2D);
    const blob = vi.spyOn(HTMLCanvasElement.prototype, "toBlob")
      .mockImplementation(callback => callback(new Blob(["replacement"], { type: "image/jpeg" })));
    const view = render(<DeckResult slides={[{
      title: "Обложка", points: ["Тезис"], image: "data:image/jpeg;base64,cHJldmlldw==",
      imageFrame: { x: 2500, y: 1000, width: 5000, height: 4000 },
    }]} brief={brief} appearance={defaultDeckAppearance} onAppearanceChange={vi.fn()} onChange={onChange} onNew={vi.fn()} />);
    try {
      const input = view.container.querySelector('input[type="file"]')!;
      fireEvent.change(input, { target: { files: [new File(["replacement"], "photo.png", { type: "image/png" })] } });
      expect(decode).toHaveBeenCalledTimes(1);
      view.unmount();
      finishDecode();
      await waitFor(() => expect(revoke).toHaveBeenCalledWith("blob:epifera-cancelled"));
      expect(onChange).not.toHaveBeenCalled();
    } finally {
      view.unmount();
      canvas.mockRestore();
      blob.mockRestore();
      vi.unstubAllGlobals();
    }
  });

  it("reveals points one step at a time and restores the last step when going back", () => {
    render(<Editor />);
    fireEvent.click(screen.getByRole("button", { name: "Начать показ" }));
    const show = within(screen.getByRole("dialog", { name: brief.topic }));
    const currentSlide = () => show.getByRole("article");
    const points = () => currentSlide().querySelectorAll("li");
    expect(currentSlide()).toHaveTextContent("Италия");
    expect(points()[0]).toHaveAttribute("aria-hidden", "true");

    fireEvent.click(show.getByRole("button", { name: "Следующий шаг" }));
    expect(points()[0]).not.toHaveAttribute("aria-hidden", "true");
    fireEvent.keyDown(window, { key: "ArrowRight" });
    expect(currentSlide()).toHaveTextContent("Встреча");
    expect(points()[0]).toHaveAttribute("aria-hidden", "true");

    fireEvent.click(show.getByRole("button", { name: "Предыдущий шаг" }));
    expect(currentSlide()).toHaveTextContent("Италия");
    expect(points()[0]).not.toHaveAttribute("aria-hidden", "true");
  });

  it("uses the configured colors and font during the show and advances directly when animation is off", () => {
    render(<Editor initialAppearance={{
      ...defaultDeckAppearance, font: "georgia", animation: "off", transition: "push",
      colors: { background: "#123456", text: "#fedcba" },
    }} />);
    fireEvent.click(screen.getByRole("button", { name: "Начать показ" }));
    const show = within(screen.getByRole("dialog", { name: brief.topic }));
    const first = show.getByRole("article");
    expect(first).toHaveStyle("background-color: #123456; color: #fedcba; font-family: Georgia, serif");
    expect(show.getByRole("listitem")).toHaveTextContent("Обложка");
    fireEvent.click(show.getByRole("button", { name: "Следующий шаг" }));
    const next = show.getByRole("article");
    expect(next).toHaveTextContent("Встреча");
    expect(next.closest("[data-transition]")).toHaveAttribute("data-transition", "push");
    expect(show.getByRole("listitem")).toHaveTextContent("Тезис");
  });

  it("uses changed transition settings on the next slide change without replaying the previous one", () => {
    render(<Editor initialAppearance={{ ...defaultDeckAppearance, animation: "off" }} />);
    fireEvent.click(screen.getByRole("button", { name: "Начать показ" }));
    const show = within(screen.getByRole("dialog", { name: brief.topic }));
    fireEvent.click(show.getByRole("button", { name: "Следующий шаг" }));
    const currentStage = () => show.getByRole("article").closest("[data-transition]");
    expect(currentStage()).toHaveAttribute("data-transition", "fade");

    fireEvent.click(show.getByRole("button", { name: "Настройки показа" }));
    const settings = within(screen.getByRole("dialog", { name: "Настройки показа" }));
    fireEvent.click(settings.getByRole("button", { name: "Сдвиг" }));
    expect(currentStage()).toHaveAttribute("data-transition", "none");
    expect(show.getByRole("article")).toHaveTextContent("Встреча");

    fireEvent.click(show.getByRole("button", { name: "Предыдущий шаг" }));
    expect(currentStage()).toHaveAttribute("data-transition", "push");
  });

  it("saves the added slide, moves it, and can undo its removal", () => {
    render(<Editor />);
    const outline = screen.getByRole("navigation", { name: "Слайды презентации" });
    fireEvent.click(screen.getByRole("button", { name: "Добавить слайд" }));
    fireEvent.change(screen.getByRole("textbox", { name: "Заголовок слайда" }), { target: { value: "План поездки" } });
    fireEvent.change(screen.getByRole("textbox", { name: "Тезисы, каждый с новой строки" }), { target: { value: "Посетить Турин" } });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить слайд" }));
    expect(within(outline).getAllByRole("button").map(button => button.textContent)).toEqual(["01Италия", "02План поездки", "03Встреча"]);
    fireEvent.click(screen.getByRole("button", { name: "Переместить позже" }));
    expect(within(outline).getAllByRole("button").map(button => button.textContent)).toEqual(["01Италия", "02Встреча", "03План поездки"]);
    fireEvent.click(screen.getByRole("button", { name: "Удалить слайд" }));
    expect(within(outline).queryByRole("button", { name: /План поездки/ })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Отменить изменение" }));
    expect(within(outline).getByRole("button", { name: /План поездки/ })).toBeInTheDocument();
  });
});
