import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { DeckResult } from "./DeckResult";
import { emptyBrief, type DeckSlide } from "./deck-content";

vi.mock("@/features/landing-variants/third/HomeMedia", () => ({
  HomePicture: () => <span />,
}));

const brief = { ...emptyBrief, topic: "Поездка по Италии" };

function Editor() {
  const [slides, setSlides] = useState<DeckSlide[]>([
    { title: "Италия", points: ["Обложка"] },
    { title: "Встреча", points: ["Тезис"] },
  ]);
  return <DeckResult slides={slides} brief={brief} onChange={setSlides} onNew={() => {}} />;
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
