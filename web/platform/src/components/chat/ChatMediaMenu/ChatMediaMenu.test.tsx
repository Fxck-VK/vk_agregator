import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ChatMediaMenu } from "./ChatMediaMenu";

const labels = {
  chooseGenerated: "Выбрать из сгенерированных",
  chooseUploaded: "Выбрать из загруженных",
  menu: "Добавить медиа",
  trigger: "Загрузить медиа",
  uploadFile: "Загрузить файл",
};

describe("ChatMediaMenu", () => {
  afterEach(() => {
    cleanup();
  });

  it("opens the requested media source actions", () => {
    render(<ChatMediaMenu labels={labels} />);

    fireEvent.click(screen.getByRole("button", { name: labels.trigger }));

    expect(screen.getByRole("menu", { name: labels.menu })).toBeVisible();
    expect(screen.getByRole("menuitem", { name: labels.uploadFile })).toBeVisible();
    expect(screen.getByRole("menuitem", { name: labels.chooseUploaded })).toHaveAttribute(
      "href",
      "/app/files?category=uploads",
    );
    expect(screen.getByRole("menuitem", { name: labels.chooseGenerated })).toHaveAttribute(
      "href",
      "/app/files?category=images",
    );
  });

  it("returns files from the native file picker", () => {
    const onFilesSelected = vi.fn();
    const { container } = render(<ChatMediaMenu labels={labels} onFilesSelected={onFilesSelected} />);
    const file = new File(["image"], "reference.png", { type: "image/png" });

    fireEvent.click(screen.getByRole("button", { name: labels.trigger }));
    fireEvent.click(screen.getByRole("menuitem", { name: labels.uploadFile }));
    const input = container.querySelector('input[type="file"]');
    expect(input).not.toBeNull();
    fireEvent.change(input as HTMLInputElement, { target: { files: [file] } });

    expect(onFilesSelected).toHaveBeenCalledWith([file]);
    expect(screen.queryByRole("menu", { name: labels.menu })).not.toBeInTheDocument();
  });

  it("calls the library actions and closes the menu", () => {
    const onChooseGenerated = vi.fn();
    const onChooseUploaded = vi.fn();
    render(
      <ChatMediaMenu
        labels={labels}
        onChooseGenerated={onChooseGenerated}
        onChooseUploaded={onChooseUploaded}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: labels.trigger }));
    fireEvent.click(screen.getByRole("menuitem", { name: labels.chooseUploaded }));
    expect(onChooseUploaded).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("menu", { name: labels.menu })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: labels.trigger }));
    fireEvent.click(screen.getByRole("menuitem", { name: labels.chooseGenerated }));
    expect(onChooseGenerated).toHaveBeenCalledTimes(1);
  });

  it("closes on Escape and outside click", () => {
    render(
      <div>
        <ChatMediaMenu labels={labels} />
        <button type="button">Снаружи</button>
      </div>,
    );

    fireEvent.click(screen.getByRole("button", { name: labels.trigger }));
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("menu", { name: labels.menu })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: labels.trigger }));
    fireEvent.pointerDown(screen.getByRole("button", { name: "Снаружи" }));
    expect(screen.queryByRole("menu", { name: labels.menu })).not.toBeInTheDocument();
  });
});
