import { expect, it } from "vitest";
import fixture from "../session/model-catalog.preview.json";
import { parseModelCatalog, projectChatModelCatalog, projectImageModelCatalog, projectVideoModelCatalog } from "./model-catalog-contract";

it("parses the backend-generated fixture and projects every available purpose", () => {
  const catalog = parseModelCatalog(fixture);
  expect(projectChatModelCatalog(catalog).items.length).toBeGreaterThan(0);
  expect(projectImageModelCatalog(catalog).items.length).toBeGreaterThan(0);
  expect(projectVideoModelCatalog(catalog).items.length).toBeGreaterThan(0);
  expect(catalog.items.some(model => model.verification === "pending-verification")).toBe(true);
  expect(catalog.items
    .filter(model => model.verification !== "pending-verification")
    .every(model => model.verification === "legacy-unverified" || model.verification === "verified-contract")).toBe(true);
  const variants = projectVideoModelCatalog(catalog).items.flatMap(model => model.variants ?? []);
  expect(variants.length).toBeGreaterThan(0);
  expect(variants.every(variant => variant.fps === null && variant.audio === null)).toBe(true);
});

it("keeps the five APIMart candidates outside generation selectors until admission", () => {
  const catalog = parseModelCatalog(fixture);
  const ids = ["nano_banana", "grok_imagine_1_5_video", "kling_2_6", "seedance_2_0", "seedance_2_0_mini"];
  const selectable = [
    ...projectImageModelCatalog(catalog).items,
    ...projectVideoModelCatalog(catalog).items,
  ];
  for (const id of ids) {
    const model = catalog.items.find(item => item.id === id);
    expect(model?.verification).toBe("pending-verification");
    expect(model?.operations.every(operation => !operation.enabled)).toBe(true);
    expect(selectable.some(item => item.id === id)).toBe(false);
  }
  expect(selectable.some(item => item.id === "video_seedance_2_0_fast")).toBe(true);
});
