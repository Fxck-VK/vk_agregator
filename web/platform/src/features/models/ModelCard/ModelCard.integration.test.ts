import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

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
    expect(presentation).toContain("/app/chats?model=${encodeURIComponent(model.id)}");
    expect(card).toContain("getModelPresentation(model)");
    expect(selector).toContain('<ModelSelectorOption');
    expect(selectorOption).toContain('<ModelCard');
    expect(selectorOption).toContain('variant="selector"');
    expect(selector).toContain("getModelPresentation(selectedModel)");
    expect(selector).toContain("src={selectedModelPresentation?.artworkSrc}");
    expect(selector).not.toMatch(
      /styles\.(?:option|optionSelected|optionIcon|optionCopy|optionTitle|optionDescription)\b/,
    );
    expect(workspaceSelector).toContain("<ModelSelector");
    expect(shortcuts).not.toContain("NeiroHub Chat");
    expect(shortcuts).not.toContain("artworkByModelId");
  });
});
