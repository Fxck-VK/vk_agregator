import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GenerationCatalogProvider, useGenerationCatalog } from "./GenerationCatalogProvider";
import { loadGenerationModelCatalog, type GenerationModelCatalog } from "./generation-model-catalog";
import { ReadError } from "@/lib/web-api/read-error";
vi.mock("./generation-model-catalog", () => ({ loadGenerationModelCatalog: vi.fn() }));
const seed: GenerationModelCatalog = { items: [{ id: "chat", name: "Seeded model", category: "text" }], default_model_id: "chat", categoryErrors: {} };
function Reader() {
  const { catalog, status, retry, failed } = useGenerationCatalog();
  return <><span>{catalog?.items[0]?.name ?? status}</span><button onClick={retry}>refresh</button>{failed && <span>refresh failed</span>}</>;
}
afterEach(() => { cleanup(); vi.useRealTimers(); vi.resetAllMocks(); });
describe("shared generation catalog", () => {
  it("recovers a transient catalog failure in the background without clearing the seed", async () => {
    vi.useFakeTimers();
    vi.mocked(loadGenerationModelCatalog).mockRejectedValueOnce(new ReadError("unavailable", 503)).mockResolvedValueOnce({ ...seed, items: [{ ...seed.items[0], name: "Recovered model" }] });
    render(<GenerationCatalogProvider initial={seed}><Reader /></GenerationCatalogProvider>);
    fireEvent.click(screen.getByText("refresh"));
    await act(async () => {});
    expect(screen.getByText("Seeded model")).toBeInTheDocument();
    expect(loadGenerationModelCatalog).toHaveBeenCalledWith({ throwOnError: true });
    await act(async () => { await vi.advanceTimersByTimeAsync(2300); });
    expect(screen.getByText("Recovered model")).toBeInTheDocument();
    expect(loadGenerationModelCatalog).toHaveBeenCalledTimes(2);
  });
  it("renders the server seed immediately and does not fetch it again during hydration", async () => {
    expect(renderToStaticMarkup(<GenerationCatalogProvider initial={seed}><Reader /></GenerationCatalogProvider>)).toContain("Seeded model");
    render(<GenerationCatalogProvider initial={seed}><Reader /><Reader /></GenerationCatalogProvider>);
    await act(async () => {});
    expect(loadGenerationModelCatalog).not.toHaveBeenCalled();
    expect(screen.getAllByText("Seeded model")).toHaveLength(2);
  });
  it("deduplicates readers, retains the last data during failed refresh, and retries", async () => {
    vi.mocked(loadGenerationModelCatalog).mockResolvedValueOnce(seed).mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce({ ...seed, items: [{ ...seed.items[0], name: "Updated model" }] });
    render(<GenerationCatalogProvider><Reader /><Reader /></GenerationCatalogProvider>);
    await screen.findAllByText("Seeded model");
    expect(loadGenerationModelCatalog).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getAllByText("refresh")[0]);
    await screen.findAllByText("refresh failed");
    expect(screen.getAllByText("Seeded model")).toHaveLength(2);
    fireEvent.click(screen.getAllByText("refresh")[0]);
    await waitFor(() => expect(screen.getAllByText("Updated model")).toHaveLength(2));
  });
});
