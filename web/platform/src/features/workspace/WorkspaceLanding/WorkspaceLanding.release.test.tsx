import { cleanup, render } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { WorkspaceLanding } from "./WorkspaceLanding";

const state = vi.hoisted(() => ({ view: "prod", ids: ["nano_banana_2"] }));
vi.mock("@/features/release/ReleaseViewProvider", () => ({ useReleaseView: () => ({ view: state.view, enabled: () => true, allowsPath: () => true }) }));
vi.mock("@/features/models/GenerationCatalogProvider", () => ({ useGenerationCatalog: () => ({ catalog: { items: state.ids.map(id => ({ id })) } }) }));
vi.mock("../WorkspaceHero/WorkspaceHero", () => ({ WorkspaceHero: () => null }));
vi.mock("../FeaturedModels/FeaturedModels", () => ({ FeaturedModels: () => null }));
vi.mock("@/features/subscriptions/use-lite-offer", () => ({ useLiteOffer: () => ({ visible: false }) }));
afterEach(cleanup);

it("links only available catalogue models without social promotions in either release view", () => {
  const view = render(<WorkspaceLanding />);
  const agreement = view.getByRole("link", { name: "Пользовательское соглашение" });
  expect(agreement).toHaveAttribute("href", "/ru/legal/user-agreement");
  expect(agreement).toHaveAttribute("target", "_blank");
  expect(agreement).toHaveAttribute("rel", "noopener noreferrer");
  expect(view.container.querySelectorAll('footer a[href*="?model="]')).toHaveLength(1);
  expect(view.container.querySelector('[aria-disabled="true"]')).toBeNull();
  state.view = "dev";
  state.ids = ["nano_banana_2", "nano_banana_pro", "gpt_image_2", "seedream_4_5"];
  view.rerender(<WorkspaceLanding />);
  expect(view.container.querySelectorAll('footer a[href*="?model="]')).toHaveLength(4);
  expect(view.container.querySelector('[aria-disabled="true"]')).toBeNull();
  state.ids = [];
  view.rerender(<WorkspaceLanding />);
  expect(view.container.querySelectorAll('footer a[href*="?model="]')).toHaveLength(0);
});
