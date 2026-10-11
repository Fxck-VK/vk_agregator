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
afterEach(() => { vi.useRealTimers(); vi.resetAllMocks(); });

describe("server and browser catalog sharing", () => {
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
