import { StrictMode, useEffect, useState } from "react";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup, renderToString } from "react-dom/server";
import { hydrateRoot } from "react-dom/client";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { GenerationCatalogProvider, useGenerationCatalog } from "./GenerationCatalogProvider";
import { projectGenerationModelCatalog } from "./generation-model-catalog";
import { loadImageModelCatalog } from "./image-model-catalog-cache";
import { loadVideoModelCatalog } from "./video-model-catalog";
import { loadModelCatalog } from "./model-catalog-cache";
import { resetModelCatalogCacheForTests } from "./model-catalog-cache";
import { readModelCatalogSession, writeModelCatalogSession } from "./model-catalog-session-cache";
import type { ModelCatalogDelivery } from "./model-catalog-contract";
import { parseModelCatalog } from "./model-catalog-contract";
import { publicModelCatalog } from "./model-catalog-test-fixtures";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));
const catalog = parseModelCatalog(publicModelCatalog());
function SelectorReader() {
  const { catalog, status, failed, retry } = useGenerationCatalog();
  return <><span>{catalog?.items[0]?.name ?? status}</span><button onClick={retry}>retry catalog</button>{failed && <span>catalog failed</span>}</>;
}
function AdapterReader() {
  const [count, setCount] = useState<number | null>(null);
  useEffect(() => {
    let active = true;
    void Promise.all([loadModelCatalog(), loadImageModelCatalog(), loadVideoModelCatalog()])
      .then(([raw, images, video]) => { if (active) setCount(raw.items.length + images.items.length + video.items.length); });
    return () => { active = false; };
  }, []);
  return <span>{count === null ? "adapters loading" : `adapters ready: ${count}`}</span>;
}
afterEach(() => { resetModelCatalogCacheForTests(); vi.useRealTimers(); vi.resetAllMocks(); });

describe("server and browser catalog sharing", () => {
  it("keeps subscribers when the same account receives a new streamed promise", async () => {
    const view = render(<GenerationCatalogProvider accountId="same" pendingSource={Promise.resolve({ snapshot: { value: catalog, updatedAt: Date.now() }, durationMs: 5 })}><SelectorReader /></GenerationCatalogProvider>);
    await screen.findByText("Image");
    const next = structuredClone(catalog); next.items[0].name = "Updated same account";
    view.rerender(<GenerationCatalogProvider accountId="same" pendingSource={Promise.resolve({ snapshot: { value: next, updatedAt: Date.now() + 1 }, durationMs: 5 })}><SelectorReader /></GenerationCatalogProvider>);
    expect(await screen.findByText("Updated same account")).toBeInTheDocument();
    expect(webBrowserFetch).not.toHaveBeenCalled();
  });
  it("invalidates a previous account's pending server delivery before adopting the next", async () => {
    let finishA!: (delivery: ModelCatalogDelivery) => void, finishB!: (delivery: ModelCatalogDelivery) => void;
    const pendingA = new Promise<ModelCatalogDelivery>(resolve => { finishA = resolve; });
    const pendingB = new Promise<ModelCatalogDelivery>(resolve => { finishB = resolve; });
    const view = render(<GenerationCatalogProvider key="A" accountId="A" pendingSource={pendingA}><SelectorReader /></GenerationCatalogProvider>);
    await act(async () => {});
    view.rerender(<GenerationCatalogProvider key="B" accountId="B" pendingSource={pendingB}><SelectorReader /></GenerationCatalogProvider>);
    await act(async () => {});
    const next = structuredClone(catalog); next.items[0].name = "Next session catalogue";
    await act(async () => { finishB({ snapshot: { value: next, updatedAt: Date.now() }, durationMs: 20 }); });
    expect(screen.getByText("Next session catalogue")).toBeInTheDocument();
    await act(async () => { finishA({ snapshot: { value: catalog, updatedAt: Date.now() }, durationMs: 300 }); });
    expect(screen.getByText("Next session catalogue")).toBeInTheDocument();
    expect(readModelCatalogSession()?.value.items[0].name).toBe("Next session catalogue");
    expect(webBrowserFetch).not.toHaveBeenCalled();
  });
  it("keeps the original TTL when restoring session storage", async () => {
    vi.useFakeTimers(); vi.setSystemTime(100_000);
    writeModelCatalogSession({ value: catalog, updatedAt: 40_001 });
    const updated = structuredClone(catalog); updated.items[0].name = "Fresh after storage TTL";
    vi.mocked(webBrowserFetch).mockImplementation(async () => Response.json(updated));
    render(<GenerationCatalogProvider><SelectorReader /></GenerationCatalogProvider>);
    await act(async () => {});
    expect(screen.getByText("Image")).toBeInTheDocument();
    expect(webBrowserFetch).not.toHaveBeenCalled();
    await act(async () => { vi.advanceTimersByTime(1); window.dispatchEvent(new Event("focus")); });
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);
    expect(screen.getByText("Fresh after storage TTL")).toBeInTheDocument();
  });

  it("shares a delayed server result with selectors and adapters without a duplicate browser request", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(catalog));
    let finish!: (result: ModelCatalogDelivery) => void;
    const pending = new Promise<ModelCatalogDelivery>(resolve => { finish = resolve; });
    render(<GenerationCatalogProvider pendingSource={pending}><SelectorReader /><AdapterReader /></GenerationCatalogProvider>);
    await act(async () => {});
    expect(webBrowserFetch).not.toHaveBeenCalled();
    finish({ snapshot: { value: catalog, updatedAt: Date.now() }, durationMs: 300 });
    await screen.findByText("adapters ready: 5");
    expect(screen.getByText("Image")).toBeInTheDocument();
    expect(webBrowserFetch).not.toHaveBeenCalled();
  });
  it("restores the saved presentation after hydration while a server refresh is pending", async () => {
    resetModelCatalogCacheForTests();
    vi.mocked(webBrowserFetch).mockResolvedValue(Response.json(catalog));
    writeModelCatalogSession({ value: catalog, updatedAt: Date.now() - 61_000 });
    let finish!: (result: ModelCatalogDelivery) => void;
    const pending = new Promise<ModelCatalogDelivery>(resolve => { finish = resolve; });
    const view = <GenerationCatalogProvider pendingSource={pending}><SelectorReader /></GenerationCatalogProvider>;
    expect(renderToString(view)).toContain("loading");
    render(view);
    expect(await screen.findByText("Image")).toBeInTheDocument();
    expect(webBrowserFetch).not.toHaveBeenCalled();
    const fresh = structuredClone(catalog); fresh.items[0].name = "Updated server image";
    finish({ snapshot: { value: fresh, updatedAt: Date.now() }, durationMs: 300 });
    expect(await screen.findByText("Updated server image")).toBeInTheDocument();
  });
  it("reuses the server snapshot before child adapters run, without fetching it again", async () => {
    vi.mocked(webBrowserFetch).mockImplementation(async () => Response.json(catalog));
    render(<GenerationCatalogProvider initial={projectGenerationModelCatalog(catalog)} initialSource={{ value: catalog, updatedAt: Date.now() }}>
      <SelectorReader /><AdapterReader />
    </GenerationCatalogProvider>);
    await screen.findByText("adapters ready: 5");
    expect(screen.getByText("Image")).toBeInTheDocument();
    expect(webBrowserFetch).not.toHaveBeenCalled();
  });

  it("shares one request across selectors and adapters when the server has no seed", async () => {
    let finish!: (response: Response) => void;
    vi.mocked(webBrowserFetch).mockReturnValue(new Promise(resolve => { finish = resolve; }));
    render(<GenerationCatalogProvider><SelectorReader /><AdapterReader /></GenerationCatalogProvider>);
    await act(async () => {});
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);
    finish(Response.json(catalog));
    await screen.findByText("adapters ready: 5");
    expect(screen.getByText("Image")).toBeInTheDocument();
  });

  it("hydrates the actual server markup in StrictMode without a duplicate catalog request", async () => {
    vi.mocked(webBrowserFetch).mockImplementation(async () => Response.json(catalog));
    const view = <StrictMode><GenerationCatalogProvider initialSource={{ value: catalog, updatedAt: Date.now() }}>
      <SelectorReader /><AdapterReader />
    </GenerationCatalogProvider></StrictMode>;
    const container = document.createElement("div");
    container.innerHTML = renderToString(view);
    document.body.append(container);
    let root: ReturnType<typeof hydrateRoot> | undefined;
    try {
      expect(within(container).getByText("Image")).toBeInTheDocument();
      await act(async () => { root = hydrateRoot(container, view); });
      await within(container).findByText("adapters ready: 5");
      expect(webBrowserFetch).not.toHaveBeenCalled();
    } finally {
      await act(async () => { root?.unmount(); });
      container.remove();
    }
  });

  it("refreshes an expired server seed once without clearing the visible models", async () => {
    let finish!: (response: Response) => void;
    vi.mocked(webBrowserFetch).mockReturnValue(new Promise(resolve => { finish = resolve; }));
    render(<GenerationCatalogProvider initial={projectGenerationModelCatalog(catalog)} initialSource={{ value: catalog, updatedAt: Date.now() - 60_001 }}>
      <SelectorReader /><AdapterReader />
    </GenerationCatalogProvider>);
    await act(async () => {});
    expect(screen.getByText("Image")).toBeInTheDocument();
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);
    const updated = structuredClone(catalog);
    updated.items[0].name = "Updated image";
    finish(Response.json(updated));
    await screen.findByText("Updated image");
    await screen.findByText("adapters ready: 5");
  });

  it("does not populate the browser cache during server rendering", async () => {
    vi.mocked(webBrowserFetch).mockImplementation(async () => Response.json(catalog));
    expect(renderToStaticMarkup(<GenerationCatalogProvider initialSource={{ value: catalog, updatedAt: Date.now() }}><SelectorReader /></GenerationCatalogProvider>)).toContain("Image");
    await loadModelCatalog();
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);
  });

  it("refreshes on focus when the original seed TTL ends, without renewing it at hydration", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(100_000);
    const updated = structuredClone(catalog);
    updated.items[0].name = "Updated image";
    vi.mocked(webBrowserFetch).mockImplementation(async () => Response.json(updated));
    render(<GenerationCatalogProvider initialSource={{ value: catalog, updatedAt: 40_001 }}><SelectorReader /></GenerationCatalogProvider>);
    await act(async () => {});
    expect(webBrowserFetch).not.toHaveBeenCalled();
    await act(async () => { vi.advanceTimersByTime(1); window.dispatchEvent(new Event("focus")); });
    expect(webBrowserFetch).toHaveBeenCalledTimes(1);
    expect(screen.getByText("Updated image")).toBeInTheDocument();
  });

  it("preserves models after a failed refresh and shares the manual retry with raw readers", async () => {
    let finish!: (response: Response) => void;
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(new Response(null, { status: 503 }))
      .mockReturnValueOnce(new Promise(resolve => { finish = resolve; }));
    render(<GenerationCatalogProvider initialSource={{ value: catalog, updatedAt: Date.now() - 60_001 }}><SelectorReader /></GenerationCatalogProvider>);
    await screen.findByText("catalog failed");
    expect(screen.getByText("Image")).toBeInTheDocument();
    fireEvent.click(screen.getByText("retry catalog"));
    fireEvent.click(screen.getByText("retry catalog"));
    const raw = loadModelCatalog();
    await act(async () => {});
    expect(webBrowserFetch).toHaveBeenCalledTimes(2);
    const updated = structuredClone(catalog);
    updated.items[0].name = "Recovered image";
    finish(Response.json(updated));
    await screen.findByText("Recovered image");
    expect((await raw).items[0].name).toBe("Recovered image");
    expect(screen.queryByText("catalog failed")).not.toBeInTheDocument();
  });
});
