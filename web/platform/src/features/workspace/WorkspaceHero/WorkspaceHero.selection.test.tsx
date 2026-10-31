import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { WorkspaceHero } from "./WorkspaceHero";
import { WorkspaceModelSelector } from "@/features/models/WorkspaceModelSelector/WorkspaceModelSelector";
import { WorkspaceModelSelectionProvider } from "@/features/models/WorkspaceModelSelection/WorkspaceModelSelection";
import { useGenerationCatalog } from "@/features/models/GenerationCatalogProvider";
import { loadGenerationModelCatalog, type GenerationModelCatalog } from "@/features/models/generation-model-catalog";
import { parseModelCatalog, projectImageModelCatalog } from "@/features/models/model-catalog-contract";
import { webBrowserMutation } from "@/lib/web-api/browser";
import catalogueJSON from "@/features/session/model-catalog.preview.json";
import { brandName } from "@/config/brand";
import { ru } from "@/i18n/ru";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }), useSearchParams: () => new URLSearchParams(), usePathname: () => "/ru/app" }));
vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn(), webBrowserMutation: vi.fn() }));
vi.mock("@/features/models/generation-model-catalog", () => ({ loadGenerationModelCatalog: vi.fn() }));
vi.mock("@/features/models/GenerationCatalogProvider", () => ({ useGenerationCatalog: vi.fn() }));

beforeEach(() => {
  const images = projectImageModelCatalog(parseModelCatalog(catalogueJSON)).items.map(model => ({ ...model, category: "images" as const }));
  const catalog: GenerationModelCatalog = { items: [...images, { id: "chatgpt", name: `${brandName} Chat`, description: "Chat", category: "text", categories: ["free", "text"], estimate_credits: 0 }], default_model_id: "chatgpt", categoryErrors: {} };
  vi.mocked(loadGenerationModelCatalog).mockResolvedValue(catalog);
  vi.mocked(useGenerationCatalog).mockReturnValue({ catalog, pending: false, failed: false, status: "ready", retry: vi.fn() });
});
afterEach(() => { cleanup(); vi.resetAllMocks(); });

function renderHeroWithHeader() {
  render(<WorkspaceModelSelectionProvider><WorkspaceModelSelector /><WorkspaceHero access="authenticated" allModelsLink={<span />} modelLinksClassName="models" /></WorkspaceModelSelectionProvider>);
}

it("publishes the initial home text model to the header", async () => {
  renderHeroWithHeader();
  expect(await screen.findByRole("button", { name: new RegExp(`Выбрана нейросеть ${brandName} Chat`) })).toBeVisible();
});

it("keeps the header selection equal to the image model actually submitted", async () => {
  vi.mocked(webBrowserMutation).mockReturnValue(new Promise(() => {}));
  renderHeroWithHeader();
  fireEvent.click(await screen.findByRole("button", { name: "Выбрать модель: GPT Image 2.5 Sunburst" }));
  expect(await screen.findByRole("button", { name: /Выбрана нейросеть GPT Image 2.5 Sunburst/ })).toBeVisible();
  fireEvent.change(screen.getByRole("textbox"), { target: { value: "Matching selected model" } });
  fireEvent.click(screen.getByRole("button", { name: ru.imageGeneration.generate }));
  expect(JSON.parse(vi.mocked(webBrowserMutation).mock.calls[0][1]!.body as string).model_id).toBe("gpt_image_2_5_sunburst");
});

it("updates the header again when returning to the text model", async () => {
  renderHeroWithHeader();
  fireEvent.click(await screen.findByRole("button", { name: "Выбрать модель: GPT Image 2.5 Sunburst" }));
  fireEvent.click(screen.getByRole("button", { name: `Выбрать модель: ${brandName} Chat` }));
  expect(await screen.findByRole("button", { name: new RegExp(`Выбрана нейросеть ${brandName} Chat`) })).toBeVisible();
});
