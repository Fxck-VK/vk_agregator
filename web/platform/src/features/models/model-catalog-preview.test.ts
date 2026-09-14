import { expect, it } from "vitest";
import fixture from "../session/model-catalog.preview.json";
import { parseModelCatalog, projectChatModelCatalog, projectImageModelCatalog, projectVideoModelCatalog } from "./model-catalog-contract";

it("parses the backend-generated fixture and projects every available purpose", () => {
  const catalog = parseModelCatalog(fixture);
  expect(projectChatModelCatalog(catalog).items.length).toBeGreaterThan(0);
  expect(projectImageModelCatalog(catalog).items.length).toBeGreaterThan(0);
  expect(projectVideoModelCatalog(catalog).items.length).toBeGreaterThan(0);
  expect(catalog.items.every(model => model.verification === "legacy-unverified")).toBe(true);
  const variants = projectVideoModelCatalog(catalog).items.flatMap(model => model.variants ?? []);
  expect(variants.length).toBeGreaterThan(0);
  expect(variants.every(variant => variant.fps === null && variant.audio === null)).toBe(true);
});
