import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));

import { ru } from "@/i18n/ru";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { parseConversationMessageList, type ConversationImage, type ConversationMessage } from "@/lib/web-api/contracts";
import { ConversationAssistantMessage, ConversationImageGallery } from "./ConversationImageGallery";
import { collectConversationImages } from "./conversation-images";

const conversationID = "20000000-0000-4000-8000-000000000001";
const image: ConversationImage = {
  job: {
    id: "30000000-0000-4000-8000-000000000001", status: "succeeded", prompt: "Журавль",
    model_id: "nano-banana-2", model_name: "Nano Banana 2", image_quality: "2K", cost_estimate: 70,
    created_at: "2026-09-01T12:00:00Z", updated_at: "2026-09-01T12:01:00Z",
  },
  artifact: {
    id: "40000000-0000-4000-8000-000000000001", width: 1024, height: 1536, mime_type: "image/png", size_bytes: 1024,
  },
};
const secondImage: ConversationImage = {
  job: { ...image.job, id: "30000000-0000-4000-8000-000000000002", prompt: "Облако" },
  artifact: { ...image.artifact, id: "40000000-0000-4000-8000-000000000002" },
};
const imagePath = `/web/v1/image-artifacts/${image.artifact.id}`;
const message: ConversationMessage = {
  id: "24000000-0000-4000-8000-000000000001", seq: 2, role: "assistant", rating: null,
  created_at: "2026-09-01T12:01:00Z",
  text: `Готово.\n\n![Журавль](${imagePath})\n\n[Скачать изображение](${imagePath})`,
  images: [image],
};
const secondMessage: ConversationMessage = { ...message, id: "24000000-0000-4000-8000-000000000002", seq: 4, text: "Второе фото.", images: [secondImage] };

function Gallery({ messages = [message], hasMoreBefore = false, id = conversationID }: {
  messages?: ConversationMessage[]; hasMoreBefore?: boolean; id?: string;
}) {
  return (
    <ConversationImageGallery conversationID={id} hasMoreBefore={hasMoreBefore} key={id} messages={messages}>
      {messages.map((item) => <ConversationAssistantMessage key={item.id} message={item} />)}
    </ConversationImageGallery>
  );
}

function closePreview() {
  fireEvent.click(screen.getByRole("button", { name: ru.files.closePreview }));
  fireEvent.animationEnd(screen.getByTestId("file-preview-backdrop"), { animationName: "modalBackdropOut" });
}

describe("conversation image gallery", () => {
  afterEach(() => { cleanup(); vi.resetAllMocks(); });

  it("uses the shared card and preview for the same artifact without duplicate Markdown media", () => {
    render(<Gallery />);
    expect(screen.getByText("Готово.")).toBeVisible();
    const card = screen.getByRole("article");
    expect(within(card).getByRole("img", { name: ru.files.generatedImageAlt })).toHaveAttribute("src", imagePath);
    expect(screen.getAllByRole("link")).toHaveLength(1);
    expect(screen.getByRole("link")).toHaveAttribute("href", imagePath);
    const trigger = within(card).getByRole("button", { name: "Открыть файл: Журавль" });
    fireEvent.click(trigger);
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByRole("img", { name: "Журавль" })).toHaveAttribute("src", imagePath);
    expect(within(dialog).getByRole("link", { name: ru.files.previewDownloadLabel })).toHaveAttribute("href", imagePath);
    expect(within(dialog).getByText("Nano Banana 2")).toBeVisible();
    expect(within(dialog).getByRole("button", { name: "Редактировать" })).toBeVisible();
    expect(within(dialog).queryByRole("button", { name: ru.files.previewNextFile })).toBeNull();
    expect(within(dialog).queryByTestId("file-preview-thumbnail")).toBeNull();
    closePreview();
    expect(trigger).toHaveFocus();
    expect(webBrowserFetch).not.toHaveBeenCalled();
  });

  it("orders and deduplicates images from assistant messages only", () => {
    expect(collectConversationImages([
      secondMessage, message, { ...message, seq: 6 },
      { ...message, seq: 1, role: "user", images: [secondImage] },
    ])).toEqual([image, secondImage]);
  });

  it("keeps the selected file when more messages arrive and isolates a different conversation", () => {
    const view = render(<Gallery messages={[secondMessage]} />);
    fireEvent.click(screen.getByRole("button", { name: "Открыть файл: Облако" }));
    view.rerender(<Gallery messages={[message, secondMessage]} />);
    let dialog = screen.getByRole("dialog");
    expect(dialog).toHaveAccessibleName(`${ru.files.previewDialogLabel}: Облако`);
    expect(within(dialog).getAllByTestId("file-preview-thumbnail")).toHaveLength(2);
    fireEvent.click(within(dialog).getByRole("button", { name: ru.files.previewPreviousFile }));
    expect(dialog).toHaveAccessibleName(`${ru.files.previewDialogLabel}: Журавль`);
    fireEvent.click(within(dialog).getByRole("button", { name: ru.files.previewNextFile }));
    expect(dialog).toHaveAccessibleName(`${ru.files.previewDialogLabel}: Облако`);
    view.rerender(<Gallery id="20000000-0000-4000-8000-000000000002" messages={[message]} />);
    expect(screen.queryByRole("dialog")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Открыть файл: Журавль" }));
    dialog = screen.getByRole("dialog");
    expect(within(dialog).queryByText("Облако")).toBeNull();
    expect(within(dialog).queryByRole("button", { name: ru.files.previewNextFile })).toBeNull();
  });

  it("loads all older pages of this conversation before opening and reuses them on the next open", async () => {
    vi.mocked(webBrowserFetch)
      .mockResolvedValueOnce(Response.json({ items: [secondMessage], has_more_before: true }))
      .mockResolvedValueOnce(Response.json({ items: [message], has_more_before: false }));
    const latest = { ...secondMessage, seq: 104 };
    render(<Gallery messages={[latest]} hasMoreBefore />);
    fireEvent.click(screen.getByRole("button", { name: "Открыть файл: Облако" }));
    expect(screen.getByRole("status")).toHaveTextContent(ru.conversations.imagesLoading);
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getAllByTestId("file-preview-thumbnail")).toHaveLength(2);
    expect(dialog).toHaveAccessibleName(`${ru.files.previewDialogLabel}: Облако`);
    expect(vi.mocked(webBrowserFetch).mock.calls.map(([path]) => path)).toEqual([
      `/web/v1/conversations/${conversationID}/messages?before_seq=104&limit=100`,
      `/web/v1/conversations/${conversationID}/messages?before_seq=4&limit=100`,
    ]);
    closePreview();
    fireEvent.click(screen.getByRole("button", { name: "Открыть файл: Облако" }));
    expect(screen.getByRole("dialog")).toBeVisible();
    expect(webBrowserFetch).toHaveBeenCalledTimes(2);
  });

  it("allows retry after a failed gallery load without silently opening an incomplete gallery", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null, { status: 503 }))
      .mockResolvedValueOnce(Response.json({ items: [message], has_more_before: false }));
    render(<Gallery messages={[secondMessage]} hasMoreBefore />);
    fireEvent.click(screen.getByRole("button", { name: "Открыть файл: Облако" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(ru.conversations.imagesLoadFailure);
    expect(screen.queryByRole("dialog")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Открыть файл: Облако" }));
    expect(await screen.findByRole("dialog")).toBeVisible();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("aborts an unfinished request when leaving the conversation", async () => {
    let signal: AbortSignal | undefined;
    vi.mocked(webBrowserFetch).mockImplementation((_path, init) => {
      signal = init?.signal as AbortSignal;
      return new Promise(() => {});
    });
    const view = render(<Gallery messages={[secondMessage]} hasMoreBefore />);
    await act(async () => fireEvent.click(screen.getByRole("button", { name: "Открыть файл: Облако" })));
    view.unmount();
    expect(signal?.aborted).toBe(true);
  });

  it("stops pagination if the API repeats a page without moving the cursor", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ items: [secondMessage], has_more_before: true }));
    render(<Gallery messages={[secondMessage]} hasMoreBefore />);
    fireEvent.click(screen.getByRole("button", { name: "Открыть файл: Облако" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(ru.conversations.imagesLoadFailure);
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("accepts safe shared metadata and rejects private URLs or unfinished jobs", () => {
    expect(parseConversationMessageList({ items: [message] }).items[0].images).toEqual([image]);
    expect(() => parseConversationMessageList({ items: [{ ...message, images: [{ ...image, url: "https://private.invalid/file" }] }] })).toThrow();
    expect(() => parseConversationMessageList({ items: [{ ...message, images: [{ ...image, job: { ...image.job, status: "queued" } }] }] })).toThrow();
    expect(parseConversationMessageList({ items: [{ ...message, images: undefined }] }).items[0].images).toBeUndefined();
  });
});
