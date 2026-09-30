import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: () => "/ru/app/models",
  useSearchParams: () => new URLSearchParams(),
  useRouter: () => ({ push: vi.fn() }),
}));

import fixture from "@/features/session/model-catalog.preview.json";
import { GenerationCatalogProvider } from "./GenerationCatalogProvider";
import { projectGenerationModelCatalog } from "./generation-model-catalog";
import { parseModelCatalog, projectChatModelCatalog } from "./model-catalog-contract";
import { ModelsCatalog } from "./ModelsCatalog/ModelsCatalog";

afterEach(cleanup);

it("shows every new DEV text model under Text and makes the same models available to chat", () => {
  const source = parseModelCatalog(fixture);
  const candidates = source.items.filter(model => model.kind === "text" && model.verification === "pending-verification");
  expect(candidates).toHaveLength(34);
  const pendingIds = projectGenerationModelCatalog(source).items.map(model => model.id);
  for (const model of candidates) {
    expect(pendingIds).not.toContain(model.id);
    model.verification = "dev-smoke";
    model.operations[0].enabled = true;
  }
  const catalog = projectGenerationModelCatalog(source);
  render(<GenerationCatalogProvider initial={catalog}><ModelsCatalog /></GenerationCatalogProvider>);
  fireEvent.click(screen.getByRole("tab", { name: "Текст" }));
  const panel = within(screen.getByRole("tabpanel"));
  const chat = projectChatModelCatalog(source);
  for (const model of candidates) {
    expect(panel.getByText(model.name)).toBeInTheDocument();
    expect(catalog.items.find(item => item.id === model.id)?.category).toBe("text");
    expect(chat.items.some(item => item.id === model.id)).toBe(true);
  }
});
