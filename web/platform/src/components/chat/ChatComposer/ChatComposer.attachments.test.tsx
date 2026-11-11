import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { webcrypto } from "node:crypto";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import catalog from "@/features/session/model-catalog.preview.json";
import { projectGenerationModelCatalog, type GenerationModel } from "@/features/models/generation-model-catalog";
import { parseModelCatalog } from "@/features/models/model-catalog-contract";
import { useChatAttachments } from "@/features/conversations/use-chat-attachments";
import { webBrowserMutation } from "@/lib/web-api/browser";
import { WorkspaceFileDropZone } from "../WorkspaceFileDropZone/WorkspaceFileDropZone";
import { ChatComposer } from "./ChatComposer";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserMutation: vi.fn(), webBrowserFetch: vi.fn() }));
const models = projectGenerationModelCatalog(parseModelCatalog(catalog)).items;
const image = models.find(model => model.id === "nano_banana_2") as GenerationModel;
const video = models.find(model => model.id === "video_veo_3_1_quality") as GenerationModel;
const send = vi.fn();
const selected = vi.fn();

function Composer({ model }: { model: GenerationModel }) {
  const attachments = useChatAttachments(model);
  return <WorkspaceFileDropZone><ChatComposer attachmentController={attachments} canSubmit disabled={false}
    label="Запрос" mediaLabel="Загрузить медиа" onChange={vi.fn()} onSend={send} onFilesSelected={selected}
    placeholder="Запрос" submitLabel="Отправить" value="Тестовый запрос" variant="newChat" /></WorkspaceFileDropZone>;
}

beforeEach(() => { vi.stubGlobal("crypto", webcrypto); });
afterEach(() => { cleanup(); vi.resetAllMocks(); vi.unstubAllGlobals(); });

it("disables menu, file input, paste and drop together for unsupported operations", () => {
  const { container } = render(<Composer model={video} />);
  const file = new File(["x"], "reference.png", { type: "image/png" });
  fireEvent.paste(screen.getByRole("textbox"), { clipboardData: { files: [file] } });
  const workspace = container.querySelector('[data-ui="workspace-file-drop"]')!;
  fireEvent.dragEnter(workspace, { dataTransfer: { types: ["Files"], files: [file] } });
  fireEvent.drop(workspace, { dataTransfer: { types: ["Files"], files: [file] } });
  expect(screen.queryByRole("button", { name: "Загрузить медиа" })).toBeNull();
  expect(container.querySelector('input[type="file"]')).toBeNull();
  expect(screen.queryByRole("status")).toBeNull();
  expect(screen.queryByRole("alert")).toBeNull();
  expect(selected).not.toHaveBeenCalled();
  expect(webBrowserMutation).not.toHaveBeenCalled();
});

it("retains an uploaded reference and blocks sending after switching to a text-to-video operation", async () => {
  vi.mocked(webBrowserMutation).mockResolvedValue(Response.json({ artifact_id: "aaaaaaaa-1111-4111-8111-aaaaaaaaaaaa", mime_type: "image/png", size_bytes: 1, width: 2, height: 2 }));
  const { container, rerender } = render(<Composer model={image} />);
  const input = container.querySelector('input[type="file"]')!;
  expect(input).toHaveAttribute("accept", "image/png,.png,image/jpeg,.jpg,.jpeg");
  expect(input).toHaveAttribute("multiple");
  await act(async () => fireEvent.paste(screen.getByRole("textbox"), { clipboardData: { files: [new File(["x"], "reference.png", { type: "image/png" })] } }));
  await waitFor(() => expect(screen.getByRole("button", { name: "Отправить" })).toBeEnabled());
  rerender(<Composer model={video} />);
  expect(screen.getByRole("button", { name: "Удалить файл: reference.png" })).toBeVisible();
  expect(screen.queryByRole("button", { name: "Загрузить медиа" })).toBeNull();
  expect(screen.getByRole("alert")).toHaveTextContent("Вложения не подходят выбранной модели");
  fireEvent.keyDown(screen.getByRole("textbox"), { key: "Enter" });
  expect(send).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Удалить файл: reference.png" }));
  fireEvent.keyDown(screen.getByRole("textbox"), { key: "Enter" });
  expect(send).toHaveBeenCalledTimes(1);
  expect(webBrowserMutation).toHaveBeenCalledTimes(1);
});

it("rejects unsupported paste contents without submitting or uploading", () => {
  render(<Composer model={image} />);
  fireEvent.paste(screen.getByRole("textbox"), { clipboardData: { files: [new File(["x"], "reference.pdf", { type: "application/pdf" })] } });
  expect(screen.getByRole("alert")).toHaveTextContent("Эта модель принимает PNG и JPEG");
  expect(webBrowserMutation).not.toHaveBeenCalled();
  expect(send).not.toHaveBeenCalled();
});
