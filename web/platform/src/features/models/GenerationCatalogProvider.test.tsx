import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GenerationCatalogProvider, useGenerationCatalog } from "./GenerationCatalogProvider";
import { loadGenerationModelCatalog, type GenerationModelCatalog } from "./generation-model-catalog";
vi.mock("./generation-model-catalog", () => ({ loadGenerationModelCatalog: vi.fn() }));
const seed: GenerationModelCatalog = { items: [{ id: "chat", name: "Seeded model", category: "text" }], default_model_id: "chat", categoryErrors: {} };
function Reader() {
  const { catalog, status, retry, failed } = useGenerationCatalog();
  return <><span>{catalog?.items[0]?.name ?? status}</span><button onClick={retry}>refresh</button>{failed && <span>refresh failed</span>}</>;
}
afterEach(() => { cleanup(); vi.resetAllMocks(); });
describe("shared generation catalog", () => {
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
