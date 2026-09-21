import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import catalog from "@/features/session/model-catalog.preview.json";
import { parseModelCatalog, projectImageModelCatalog } from "@/features/models/model-catalog-contract";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { useReferenceQuote } from "./use-reference-quote";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));
const model = { ...projectImageModelCatalog(parseModelCatalog(catalog)).items.find(item => item.id === "seedream_5_0_pro")!, category: "images" as const };
const options = { image_quality: model.default_quality, aspect_ratio: "16:9", output_count: 1 };
const empty = { count: 0, ready: true, artifactIds: [] as string[] };
const attached = { count: 1, ready: true, artifactIds: ["photo"] };
afterEach(() => { cleanup(); vi.resetAllMocks(); });

it("does not request or show a quote on entry, during upload or for failed uploads", () => {
  const { result, rerender } = renderHook(({ attachments }) => useReferenceQuote(model, options, attachments), { initialProps: { attachments: empty } });
  expect(result.current).toMatchObject({ active: false, ready: true, cost: undefined, error: null });
  rerender({ attachments: { count: 1, ready: false, artifactIds: [] } });
  expect(result.current).toMatchObject({ active: true, ready: false, cost: undefined, error: null });
  expect(webBrowserFetch).not.toHaveBeenCalled();
});

it("waits for uploaded artifacts before requesting their price", async () => {
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ credits: 37 }));
  const { result, rerender } = renderHook(({ attachments }) => useReferenceQuote(model, options, attachments), { initialProps: { attachments: { ...attached, ready: false } } });
  expect(webBrowserFetch).not.toHaveBeenCalled();
  rerender({ attachments: attached });
  expect(result.current.ready).toBe(false);
  await waitFor(() => expect(result.current.cost).toBe(37));
  expect(result.current.ready).toBe(true);
  expect(webBrowserFetch).toHaveBeenCalledTimes(1);
});

it("clears the failure while retrying and keeps submission blocked until success", async () => {
  let finish!: (response: Response) => void;
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null, { status: 503 }))
    .mockReturnValueOnce(new Promise(resolve => { finish = resolve; }));
  const { result } = renderHook(() => useReferenceQuote(model, options, attached));
  await waitFor(() => expect(result.current.error).not.toBeNull());
  act(() => result.current.retry());
  expect(result.current).toMatchObject({ ready: false, cost: undefined, error: null });
  await act(async () => finish(Response.json({ credits: 37 })));
  expect(result.current).toMatchObject({ ready: true, cost: 37, error: null });
});

it("discards stale responses and recalculates after removing and reattaching the same file", async () => {
  let finish!: (response: Response) => void;
  vi.mocked(webBrowserFetch).mockReturnValueOnce(new Promise(resolve => { finish = resolve; }))
    .mockResolvedValueOnce(Response.json({ credits: 21 }))
    .mockReturnValueOnce(new Promise(() => {}));
  const { result, rerender } = renderHook(({ attachments }) => useReferenceQuote(model, options, attachments), { initialProps: { attachments: attached } });
  const signal = vi.mocked(webBrowserFetch).mock.calls[0][1]?.signal;
  rerender({ attachments: { ...attached, artifactIds: ["different"] } });
  expect(signal?.aborted).toBe(true);
  await waitFor(() => expect(result.current.cost).toBe(21));
  await act(async () => finish(new Response(null, { status: 503 })));
  expect(result.current.error).toBeNull();
  rerender({ attachments: empty });
  expect(result.current.active).toBe(false);
  rerender({ attachments: { ...attached, artifactIds: ["different"] } });
  expect(result.current).toMatchObject({ ready: false, cost: undefined, error: null });
  expect(webBrowserFetch).toHaveBeenCalledTimes(3);
});

it.each([0, -1, 1.5, "37", null])("does not accept an invalid server price: %s", async credits => {
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ credits }));
  const { result } = renderHook(() => useReferenceQuote(model, options, attached));
  await waitFor(() => expect(result.current.error).not.toBeNull());
  expect(result.current.ready).toBe(false);
  expect(result.current.cost).toBeUndefined();
});
