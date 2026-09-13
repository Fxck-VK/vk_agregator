import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ru } from "@/i18n/ru";
import type { ImageJob, ImageJobResult } from "@/lib/web-api/contracts";

import { FileCard } from "./FileCard";

const succeededJob: ImageJob = {
  cost_estimate: 60,
  created_at: "2026-08-01T12:00:00Z",
  id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
  image_quality: "2K",
  model_id: "nano-banana-2",
  model_name: "Nano Banana 2",
  prompt: "Неоновый город после дождя",
  status: "succeeded",
  updated_at: "2026-08-01T12:00:00Z",
};

const result: ImageJobResult = {
  artifacts: [{
    height: 1536,
    id: "6ca96a58-a902-4f23-a92a-6726e1a0cd20",
    mime_type: "image/png",
    size_bytes: 42,
    width: 1024,
  }],
  job_id: succeededJob.id,
  status: "succeeded",
};

describe("FileCard", () => {
  afterEach(cleanup);

  it("opens a completed result from the media surface and keeps download as a separate action", () => {
    const onOpenPreview = vi.fn();

    render(
      <FileCard
        isRetrying={false}
        job={succeededJob}
        onOpenPreview={onOpenPreview}
        onRequestResult={vi.fn()}
        onRetryJob={vi.fn()}
        result={result}
        resultState="idle"
      />,
    );

    const card = screen.getByRole("article");
    const preview = within(card).getByRole("button", {
      name: `Открыть файл: ${succeededJob.prompt}`,
    });
    const download = within(card).getByRole("link", {
      name: `${ru.files.download}: ${succeededJob.prompt}`,
    });

    fireEvent.click(preview);
    expect(onOpenPreview).toHaveBeenCalledWith(succeededJob, result.artifacts[0], preview);

    expect(download).toHaveAttribute("download");
    expect(download).toHaveAttribute(
      "href",
      "/web/v1/image-artifacts/6ca96a58-a902-4f23-a92a-6726e1a0cd20",
    );
    expect(within(preview).getByRole("img", { name: ru.files.generatedImageAlt })).toHaveAttribute(
      "height",
      "1536",
    );
    expect(card.querySelector("figcaption")).toBeNull();
    expect(within(download).getByText(ru.files.download)).toBeInTheDocument();
    expect(download.querySelector("img")).toHaveAttribute(
      "src",
      "/assets/icons/ui/download-white.svg",
    );
    expect(download.querySelector("svg")).toBeNull();

    const deleteControl = within(card).getByRole("button", {
      name: "Удаление файлов пока недоступно",
    });
    expect(deleteControl).toBeDisabled();
    expect(deleteControl).toHaveAttribute("title", "Удаление файлов пока недоступно");

    const metadata = screen.getByTestId("file-card-accessible-metadata");
    expect(metadata).toHaveTextContent(ru.files.statusReady);
    expect(metadata).toHaveTextContent(succeededJob.prompt);
    expect(metadata).toHaveTextContent("Nano Banana 2 · 2K");
  });

  it("keeps retry information and metadata visible when no media exists", () => {
    const paymentJob: ImageJob = {
      ...succeededJob,
      status: "awaiting_payment",
    };

    render(
      <FileCard
        isRetrying={false}
        job={paymentJob}
        onOpenPreview={vi.fn()}
        onRequestResult={vi.fn()}
        onRetryJob={vi.fn()}
        result={null}
        resultState="idle"
      />,
    );

    expect(screen.getByText(ru.files.insufficientTokensDescription)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: ru.files.retry })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: paymentJob.prompt })).toBeInTheDocument();
    expect(screen.queryByTestId("file-card-accessible-metadata")).toBeNull();
  });
});
