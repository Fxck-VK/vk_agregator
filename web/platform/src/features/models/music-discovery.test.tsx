import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: () => "/ru/app/models",
  useSearchParams: () => new URLSearchParams(),
  useRouter: () => ({ push }),
}));
const { push } = vi.hoisted(() => ({ push: vi.fn() }));

import fixture from "@/features/session/model-catalog.preview.json";
import { ConversationModelSelector, useConversationModelSelection } from "@/features/conversations/ConversationModelSelector/ConversationModelSelector";
import { NewChatPrompt } from "@/features/workspace/WorkspacePrompt/NewChatPrompt";
import { GenerationCatalogProvider } from "./GenerationCatalogProvider";
import { projectGenerationModelCatalog } from "./generation-model-catalog";
import { parseModelCatalog } from "./model-catalog-contract";
import { ModelsCatalog } from "./ModelsCatalog/ModelsCatalog";
import { WorkspaceModelSelector } from "./WorkspaceModelSelector/WorkspaceModelSelector";

const musicIds = ["suno_v6", "suno_v6_wild", "suno_v6_mini", "lyria_3_5"];

function devCatalog() {
  const catalog = parseModelCatalog(fixture);
  for (const model of catalog.items.filter(model => musicIds.includes(model.id))) {
    model.verification = "dev-smoke";
    for (const operation of model.operations) {
      if (operation.id === "generate" && operation.music) {
        operation.enabled = true;
        delete operation.music.unavailable_reason;
      }
    }
  }
  return catalog;
}

afterEach(() => { cleanup(); vi.clearAllMocks(); window.sessionStorage.clear(); });

it("keeps all four runnable music models in the shared catalog, but excludes pending and blocked models", () => {
  const source = devCatalog();
  expect(projectGenerationModelCatalog(source).items.filter(model => model.category === "audio").map(model => model.id)).toEqual(musicIds);
  expect(projectGenerationModelCatalog(parseModelCatalog(fixture)).items.some(model => model.category === "audio")).toBe(false);
  source.items.find(model => model.id === "lyria_3_5")!.operations[0].music!.unavailable_reason = "Unavailable";
  expect(projectGenerationModelCatalog(source).items.some(model => model.id === "lyria_3_5")).toBe(false);
});

it("shows the four music cards under Audio and opens their music editors", () => {
  render(<GenerationCatalogProvider initial={projectGenerationModelCatalog(devCatalog())}><ModelsCatalog /></GenerationCatalogProvider>);
  fireEvent.click(screen.getByRole("tab", { name: "Аудио" }));
  const cards = within(screen.getByRole("tabpanel")).getAllByRole("link");
  expect(cards).toHaveLength(4);
  for (const id of musicIds) expect(cards.some(card => card.getAttribute("href") === `/ru/app/music?model=${id}`)).toBe(true);
});

it("opens the selected music model from the header", () => {
  render(<GenerationCatalogProvider initial={projectGenerationModelCatalog(devCatalog())}><WorkspaceModelSelector /></GenerationCatalogProvider>);
  fireEvent.click(screen.getByRole("button", { name: /Выбрана нейросеть/ }));
  const audio = screen.getByRole("region", { name: "Аудио" });
  fireEvent.click(within(audio).getByRole("button", { name: /Lyria/ }));
  expect(push).toHaveBeenCalledWith("/ru/app/music?model=lyria_3_5");
});

function ConversationPicker() {
  const selection = useConversationModelSelection("music-discovery");
  return <><output>{selection.selectedModelId}</output><ConversationModelSelector disabled={false} selection={selection} /></>;
}

it("does not restore or offer music in a conversation even when it is the catalog default", () => {
  const catalog = projectGenerationModelCatalog(devCatalog());
  catalog.default_model_id = "suno_v6";
  window.sessionStorage.setItem("neirohub:conversation-model:music-discovery", "lyria_3_5");
  render(<GenerationCatalogProvider initial={catalog}><ConversationPicker /></GenerationCatalogProvider>);
  expect(screen.getByRole("status")).not.toHaveTextContent(/suno|lyria/);
  fireEvent.click(screen.getByRole("button", { name: /Выбрана нейросеть/ }));
  expect(screen.queryByRole("region", { name: "Аудио" })).not.toBeInTheDocument();
});

it("rejects a music model passed directly to the new-chat URL", () => {
  render(<GenerationCatalogProvider initial={projectGenerationModelCatalog(devCatalog())}><NewChatPrompt modelId="suno_v6" /></GenerationCatalogProvider>);
  expect(screen.getByRole("alert")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Начать чат" })).toBeDisabled();
});
