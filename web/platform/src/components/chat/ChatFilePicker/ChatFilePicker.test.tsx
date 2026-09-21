import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { fetchImageFileResult, fetchImageFilesPage } from "@/features/files/FilesWorkspace/files-data";
import { ru } from "@/i18n/ru";

import { ChatFilePicker } from "./ChatFilePicker";

vi.mock("@/features/files/FilesWorkspace/files-data", async () => {
  const actual = await vi.importActual<typeof import("@/features/files/FilesWorkspace/files-data")>(
    "@/features/files/FilesWorkspace/files-data",
  );

  return {
    ...actual,
    fetchImageFileResult: vi.fn(),
    fetchImageFilesPage: vi.fn(),
  };
});

const jobID = "11111111-1111-4111-8111-111111111111";
const artifactID = "22222222-2222-4222-8222-222222222222";

describe("ChatFilePicker", () => {
  beforeEach(() => {
    vi.mocked(fetchImageFilesPage).mockResolvedValue({
      has_more: false,
      items: [{
        cost_estimate: 55,
        created_at: "2026-08-19T09:00:00.000Z",
        id: jobID,
        image_quality: "2K",
        model_id: "nano-banana-2",
        model_name: "Nano Banana 2",
        prompt: "Город после дождя",
        status: "succeeded",
        updated_at: "2026-08-19T09:01:00.000Z",
      }],
      next_cursor: null,
    });
    vi.mocked(fetchImageFileResult).mockResolvedValue({
      artifacts: [{
        height: 1024,
        id: artifactID,
        mime_type: "image/png",
        size_bytes: 4096,
        width: 1024,
      }],
      job_id: jobID,
      status: "succeeded",
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("opens on generated files and returns the selected artifact", async () => {
    const onSelect = vi.fn();
    render(<ChatFilePicker initialSource="generated" onClose={vi.fn()} onSelect={onSelect} />);

    expect(screen.getByRole("dialog", { name: "Мои файлы" })).toBeVisible();
    expect(screen.getByRole("tab", { name: "Сгенерированные" })).toHaveAttribute("aria-selected", "true");

    fireEvent.click(await screen.findByRole("button", { name: "Прикрепить «Город после дождя»" }));

    expect(onSelect).toHaveBeenCalledWith({
      id: artifactID,
      mimeType: "image/png",
      name: "Город после дождя",
      previewUrl: `/web/v1/image-artifacts/${artifactID}`,
      source: "generated",
    });
  });

  it("lets the user retry a failed preview before attaching it", async () => {
    vi.mocked(fetchImageFileResult).mockRejectedValueOnce(new Error("Unavailable"));
    const onSelect = vi.fn();
    render(<ChatFilePicker initialSource="generated" onClose={vi.fn()} onSelect={onSelect} />);

    const error = await screen.findByRole("alert");
    expect(error).toHaveTextContent(ru.files.previewFailure);
    expect(screen.queryByRole("button", { name: /Прикрепить/ })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Повторить/ }));
    fireEvent.click(await screen.findByRole("button", { name: "Прикрепить «Город после дождя»" }));

    expect(fetchImageFileResult).toHaveBeenCalledTimes(2);
    expect(onSelect).toHaveBeenCalledOnce();
  });

  it("opens on uploaded files and lets the user upload from the empty state", () => {
    const onSelect = vi.fn();
    render(
      <ChatFilePicker initialSource="uploaded" onClose={vi.fn()} onSelect={onSelect} />,
    );

    expect(screen.getByRole("tab", { name: "Загруженные" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText("Подходящих загруженных файлов пока нет")).toBeVisible();

    const file = new File(["image"], "reference.png", { type: "image/png" });
    const input = screen.getByLabelText("Загрузить файл", { selector: 'input[type="file"]' });
    expect(input).not.toBeNull();
    fireEvent.change(input as HTMLInputElement, { target: { files: [file] } });

    expect(onSelect).toHaveBeenCalledWith(expect.objectContaining({
      file,
      mimeType: "image/png",
      name: "reference.png",
      source: "uploaded",
    }));
  });

  it("closes on Escape and restores page scrolling", async () => {
    const onClose = vi.fn();
    render(<ChatFilePicker initialSource="generated" onClose={onClose} onSelect={vi.fn()} />);

    expect(document.body.style.overflow).toBe("hidden");
    const backdrop = screen.getByRole("dialog").closest("[data-state]")!;
    fireEvent.keyDown(document, { key: "Escape" });

    expect(onClose).not.toHaveBeenCalled();
    fireEvent.animationEnd(backdrop, { animationName: "modalBackdropOut" });
    expect(onClose).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(document.body.style.overflow).toBe(""));
  });
});
