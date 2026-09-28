import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";
import { getModelPresentation } from "./model-card-content";

function readSource(path: string) {
  return readFileSync(resolve(process.cwd(), path), "utf8");
}

describe("shared model-card integration", () => {
  it("keeps model content, artwork, and route in one presentation resolver", () => {
    const presentation = readSource(
      "src/features/models/ModelCard/model-card-content.ts",
    );
    const card = readSource("src/features/models/ModelCard/ModelCard.tsx");
    const selector = readSource(
      "src/features/models/WorkspaceModelSelector/ModelSelector.tsx",
    );
    const selectorOption = readSource(
      "src/features/models/WorkspaceModelSelector/ModelSelectorOption.tsx",
    );
    const workspaceSelector = readSource(
      "src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx",
    );
    const shortcuts = readSource(
      "src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.tsx",
    );

    expect(presentation).toContain("getModelPresentation");
    expect(presentation).not.toContain("modelPresentationById");
    expect(getModelPresentation({ id: "image/1", name: "Image", category: "images" }).href).toBe("/app/chats?model=image%2F1");
    expect(getModelPresentation({ id: "suno_v6", name: "Suno", category: "audio" }).href).toBe("/app/music?model=suno_v6");
    expect(card).toContain("getModelPresentation(model, msg)");
    expect(selector).toContain('<ModelSelectorOption');
    expect(selectorOption).toContain('<ModelCard');
    expect(selectorOption).toContain('variant="selector"');
    expect(selector).toContain("getModelPresentation(selectedModel, msg)");
    expect(selector).toContain("src={selectedModelPresentation?.artworkSrc}");
    expect(selector).not.toMatch(
      /styles\.(?:option|optionSelected|optionIcon|optionCopy|optionTitle|optionDescription)\b/,
    );
    expect(workspaceSelector).toContain("<ModelSelector");
    expect(shortcuts).not.toContain("NeiroHub Chat");
    expect(shortcuts).not.toContain("artworkByModelId");
  });
});
