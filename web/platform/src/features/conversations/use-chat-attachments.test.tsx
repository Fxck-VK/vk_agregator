import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { webcrypto } from "node:crypto";
import { attachmentFromFile } from "@/components/chat/ChatFilePicker/ChatFilePicker";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { WebNetworkError } from "@/lib/web-api/network-error";
import catalog from "@/features/session/model-catalog.preview.json";
import { parseModelCatalog, projectImageModelCatalog } from "@/features/models/model-catalog-contract";
import type { GenerationModel } from "@/features/models/generation-model-catalog";
import { useChatAttachments } from "./use-chat-attachments";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserMutation: vi.fn(), webBrowserFetch: vi.fn() }));
const modelCatalog = parseModelCatalog(catalog);
const model: GenerationModel = { ...projectImageModelCatalog(modelCatalog).items.find((item) => item.id === "nano_banana_2")!, category: "images", operations: modelCatalog.items.find((item) => item.id === "nano_banana_2")!.operations };
const id = "aaaaaaaa-1111-4111-8111-aaaaaaaaaaaa";
const file = (content = "synthetic") => attachmentFromFile(new File([content], "reference.png", { type: "image/png" }));
const uploaded = () => Response.json({ artifact_id: id, mime_type: "image/png", size_bytes: 9, width: 2, height: 2 });
beforeEach(() => { vi.stubGlobal("crypto", webcrypto); });
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.resetAllMocks(); vi.unstubAllGlobals(); });

it("uploads bytes and blocks send until the owned artifact ID arrives", async () => {
  let finish!: (response: Response) => void;
  vi.mocked(webBrowserMutation).mockReturnValue(new Promise((resolve) => { finish = resolve; }));
  const { result } = renderHook(() => useChatAttachments(model));
  const selected = file(); await act(async () => { await result.current.add([selected]); });
  expect(result.current.blocked).toBe(true);
  const [path, request] = vi.mocked(webBrowserMutation).mock.calls[0];
  expect(path).toBe("/web/v1/input-artifacts?model_id=nano_banana_2");
  expect((request.body as FormData).get("file")).toBe(selected.file);
  act(() => request.onUploadProgress?.(45));
  expect(result.current.items[0].progress).toBe(45);
  act(() => request.onUploadProgress?.(100));
  expect(result.current.items[0].status).toBe("uploading");
  expect(result.current.blocked).toBe(true);
  await act(async () => finish(uploaded()));
  expect(result.current.ids).toEqual([id]); expect(result.current.blocked).toBe(false);
});

it("rejects unsupported models, file types and excess count without uploading", async () => {
  const { result, rerender } = renderHook(({ selected }) => useChatAttachments(selected), { initialProps: { selected: undefined as GenerationModel | undefined } });
  act(() => result.current.add([file()])); expect(result.current.error).toContain("не поддерживает");
  rerender({ selected: model });
  act(() => result.current.add([attachmentFromFile(new File(["doc"], "file.pdf", { type: "application/pdf" }))]));
  expect(result.current.error).toContain("PNG и JPEG");
  await act(async () => { await result.current.add(Array.from({ length: model.max_reference_images + 1 }, (_, index) => file(String(index)))); });
  expect(result.current.error).toContain("не более"); expect(webBrowserMutation).not.toHaveBeenCalled();
});

it("clears a rejected model's notice when switching models and accepts a supported reference", async () => {
  vi.mocked(webBrowserMutation).mockResolvedValue(uploaded());
  const { result, rerender } = renderHook(({ selected }) => useChatAttachments(selected), { initialProps: { selected: undefined as GenerationModel | undefined } });
  await act(async () => { await result.current.add([file()]); });
  expect(result.current.error).toContain("не поддерживает");
  rerender({ selected: model });
  expect(result.current.error).toBeNull();
  rerender({ selected: undefined });
  expect(result.current.error).toBeNull();
  rerender({ selected: model });
  await act(async () => { await result.current.add([file()]); });
  await waitFor(() => expect(result.current.ids).toEqual([id]));
  expect(result.current.blocked).toBe(false);
});

it("retains a failed file for retry and prevents incompatible model switches from sending", async () => {
  vi.mocked(webBrowserMutation).mockResolvedValueOnce(new Response(null, { status: 503 })).mockResolvedValueOnce(uploaded());
  const { result, rerender } = renderHook(({ selected }) => useChatAttachments(selected), { initialProps: { selected: model as GenerationModel | undefined } });
  const selected = file(); await act(async () => { await result.current.add([selected]); });
  await waitFor(() => expect(result.current.items[0].status).toBe("failed"));
  expect(result.current.networkNotice).toBe(false);
  expect(result.current.blocked).toBe(true);
  act(() => result.current.retry(selected.id));
  await waitFor(() => expect(result.current.ids).toEqual([id]));
  rerender({ selected: undefined }); expect(result.current.blocked).toBe(true);
  act(() => result.current.remove(selected.id)); expect(result.current.blocked).toBe(false);
});

it("shows one dismissible network notice, preserves local photos and clears it after recovery", async () => {
  vi.mocked(webBrowserMutation).mockRejectedValue(new WebNetworkError());
  const { result } = renderHook(() => useChatAttachments(model));
  await act(async () => { await result.current.add([file(), file("second")]); });
  await waitFor(() => expect(result.current.items.every(item => item.status === "failed")).toBe(true));
  expect(result.current.networkNotice).toBe(true);
  expect(result.current.blocked).toBe(true);
  expect(result.current.items).toHaveLength(2);
  act(() => result.current.dismissNetworkNotice());
  expect(result.current.networkNotice).toBe(false);
  const selected = result.current.items[0];
  act(() => result.current.retry(selected.id));
  await waitFor(() => expect(result.current.networkNotice).toBe(true));
  act(() => result.current.remove(result.current.items[1].id));
  vi.mocked(webBrowserMutation).mockResolvedValue(uploaded());
  act(() => result.current.retry(selected.id));
  await waitFor(() => expect(result.current.items[0].status).toBe("ready"));
  expect(result.current.networkNotice).toBe(false);
  expect(result.current.blocked).toBe(false);
});

it("does not turn a canceled upload into a network notice", async () => {
  let fail!: (reason: unknown) => void;
  vi.mocked(webBrowserMutation).mockReturnValue(new Promise((_resolve, reject) => { fail = reject; }));
  const { result } = renderHook(() => useChatAttachments(model));
  await act(async () => { await result.current.add([file()]); });
  act(() => result.current.clear());
  await act(async () => fail(new WebNetworkError()));
  expect(result.current.networkNotice).toBe(false);
  expect(result.current.items).toEqual([]);
});

it("aborts removed uploads and ignores late responses", async () => {
  let finish!: (response: Response) => void;
  vi.mocked(webBrowserMutation).mockReturnValue(new Promise((resolve) => { finish = resolve; }));
  const { result } = renderHook(() => useChatAttachments(model));
  const selected = file(); await act(async () => { await result.current.add([selected]); });
  const signal = vi.mocked(webBrowserMutation).mock.calls[0][1].signal!;
  act(() => result.current.remove(selected.id)); expect(signal.aborted).toBe(true);
  await act(async () => finish(uploaded()));
  expect(result.current.items).toEqual([]); expect(result.current.ids).toEqual([]);
});

it("does not send the same stored image twice after server deduplication", async () => {
  vi.mocked(webBrowserMutation).mockImplementation(async () => uploaded());
  const { result } = renderHook(() => useChatAttachments(model));
  await act(async () => { await result.current.add([file(), file("different bytes, same server artifact")]); });
  await waitFor(() => expect(result.current.items).toHaveLength(1));
  expect(result.current.ids).toEqual([id]);
  expect(webBrowserMutation).toHaveBeenCalledTimes(2);
  expect(result.current.duplicateNotice).toBe(true);
});

it("rejects renamed copies before upload, including duplicates selected together or concurrently", async () => {
  vi.mocked(webBrowserMutation).mockReturnValue(new Promise(() => {}));
  const { result } = renderHook(() => useChatAttachments(model));
  const copy = () => attachmentFromFile(new File(["synthetic"], "renamed.png", { type: "image/png", lastModified: 123 }));
  await act(async () => {
    await Promise.all([result.current.add([file(), copy()]), result.current.add([copy()])]);
  });
  expect(webBrowserMutation).toHaveBeenCalledTimes(1);
  expect(result.current.items).toHaveLength(1);
  expect(result.current.duplicateNotice).toBe(true);
  act(() => result.current.dismissDuplicateNotice());
  expect(result.current.duplicateNotice).toBe(false);
});

it("keeps different contents with identical names, sizes and timestamps", async () => {
  vi.mocked(webBrowserMutation).mockReturnValue(new Promise(() => {}));
  const { result } = renderHook(() => useChatAttachments(model));
  const sameMetadata = (body: string) => attachmentFromFile(new File([body], "same.png", { type: "image/png", lastModified: 123 }));
  await act(async () => { await result.current.add([sameMetadata("AAA"), sameMetadata("BBB")]); });
  expect(webBrowserMutation).toHaveBeenCalledTimes(2);
  expect(result.current.items).toHaveLength(2);
  expect(result.current.duplicateNotice).toBe(false);
});

it("checks duplicates before the model count limit and allows adding again after removal", async () => {
  vi.mocked(webBrowserMutation).mockReturnValue(new Promise(() => {}));
  const { result } = renderHook(() => useChatAttachments({ ...model, max_reference_images: 1 }));
  await act(async () => { await result.current.add([file()]); });
  await act(async () => { await result.current.add([file()]); });
  expect(result.current.error).toBeNull();
  expect(result.current.duplicateNotice).toBe(true);
  expect(webBrowserMutation).toHaveBeenCalledTimes(1);
  act(() => result.current.remove(result.current.items[0].id));
  await act(async () => { await result.current.add([file()]); });
  expect(result.current.items).toHaveLength(1);
  expect(webBrowserMutation).toHaveBeenCalledTimes(2);
});

it("does not upload or resurrect a selection cleared while its contents are checked", async () => {
  const { result } = renderHook(() => useChatAttachments(model));
  let added: ReturnType<typeof result.current.add>;
  act(() => { added = result.current.add([file()]); });
  expect(result.current.blocked).toBe(true);
  act(() => result.current.clear());
  await act(async () => { await added; });
  expect(result.current.items).toEqual([]);
  expect(result.current.blocked).toBe(false);
  expect(webBrowserMutation).not.toHaveBeenCalled();
});

it("recognizes stored images by content and avoids refetching the same stored ID", async () => {
  vi.mocked(webBrowserMutation).mockReturnValue(new Promise(() => {}));
  vi.mocked(webBrowserFetch).mockResolvedValue({ ok: true, blob: async () => new Blob(["synthetic"], { type: "image/png" }) } as Response);
  const { result } = renderHook(() => useChatAttachments(model));
  const stored = { id, name: "stored.png", source: "generated" as const, mimeType: "image/png" };
  await act(async () => { await result.current.add([stored]); });
  await act(async () => { await result.current.add([stored, file()]); });
  expect(result.current.items).toHaveLength(1);
  expect(result.current.duplicateNotice).toBe(true);
  expect(webBrowserFetch).toHaveBeenCalledTimes(1);
  expect(webBrowserMutation).toHaveBeenCalledTimes(1);
});

it("reports unreadable contents and accepts the next queued selection", async () => {
  vi.spyOn(crypto.subtle, "digest").mockRejectedValueOnce(new Error("unreadable"));
  vi.mocked(webBrowserMutation).mockReturnValue(new Promise(() => {}));
  const { result } = renderHook(() => useChatAttachments(model));
  await act(async () => { await result.current.add([file()]); });
  expect(result.current.error).toContain("Не удалось прочитать файл");
  expect(result.current.blocked).toBe(false);
  expect(webBrowserMutation).not.toHaveBeenCalled();
  await act(async () => { await result.current.add([file()]); });
  expect(result.current.error).toBeNull();
  expect(result.current.items).toHaveLength(1);
});

it("cancels an in-flight fingerprint check on unmount without uploading", async () => {
  let finish!: (buffer: ArrayBuffer) => void;
  const digest = vi.spyOn(crypto.subtle, "digest").mockReturnValueOnce(new Promise(resolve => { finish = resolve; }));
  const { result, unmount } = renderHook(() => useChatAttachments(model));
  let selection: ReturnType<typeof result.current.add>;
  act(() => { selection = result.current.add([file()]); });
  await waitFor(() => expect(digest).toHaveBeenCalled());
  unmount();
  finish(new ArrayBuffer(32));
  await selection;
  expect(webBrowserMutation).not.toHaveBeenCalled();
});
