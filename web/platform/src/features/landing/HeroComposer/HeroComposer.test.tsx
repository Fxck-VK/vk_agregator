import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

import { LandingToolSelectionProvider, useLandingToolSelection } from "../LandingToolSelection/LandingToolSelection";
import { HeroComposer } from "./HeroComposer";

function ImageSelectionButton() {
  const { selectTool } = useLandingToolSelection();

  return <button onClick={() => selectTool("gpt-image")} type="button">Выбрать GPT Image</button>;
}

describe("HeroComposer", () => {
  afterEach(() => {
    cleanup();
    sessionStorage.clear();
    push.mockClear();
  });

  it("keeps an image-looking prompt in the universal chat until the user selects a model", () => {
    render(<LandingToolSelectionProvider><HeroComposer /></LandingToolSelectionProvider>);

    const input = screen.getByRole("textbox", { name: "Задайте вопрос NeiroHub" });
    fireEvent.change(input, { target: { value: "Создай изображение космического города" } });

    expect(screen.queryByLabelText("Качество")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Начать чат" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Добавить файл после входа" })).toHaveAttribute(
      "href",
      "/login?next=/app/chats",
    );
  });

  it("uses Enter to save the guest draft and open login while Shift+Enter keeps editing", () => {
    render(<LandingToolSelectionProvider><HeroComposer /></LandingToolSelectionProvider>);
    const input = screen.getByRole("textbox", { name: "Задайте вопрос NeiroHub" });

    fireEvent.change(input, { target: { value: "Первая строка" } });
    fireEvent.keyDown(input, { key: "Enter", shiftKey: true });
    expect(push).not.toHaveBeenCalled();

    fireEvent.keyDown(input, { key: "Enter" });
    expect(push).toHaveBeenCalledWith("/login?next=/app/chats");
    expect(sessionStorage.getItem("neirohub.guest-draft")).toContain("Первая строка");
  });

  it("shows image settings and an immediate price only after explicit selection", () => {
    render(
      <LandingToolSelectionProvider>
        <ImageSelectionButton />
        <HeroComposer />
      </LandingToolSelectionProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Выбрать GPT Image" }));

    expect(screen.getByRole("textbox", { name: "Описание изображения" })).toBeInTheDocument();
    expect(screen.getByLabelText("Качество")).toBeInTheDocument();
    expect(screen.getByText("Стоимость: 16 ★")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Создать изображение · 16 ★" })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Качество"), { target: { value: "2K" } });
    expect(screen.getByText("Стоимость: 60 ★")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Создать изображение · 60 ★" })).toBeInTheDocument();
  });
});
