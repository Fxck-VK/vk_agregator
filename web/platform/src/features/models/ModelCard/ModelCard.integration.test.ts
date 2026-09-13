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
    const workspaceSelector = readSource(
      "src/features/models/WorkspaceModelSelector/WorkspaceModelSelector.tsx",
    );
    const shortcuts = readSource(
      "src/features/workspace/FeaturedModelShortcuts/FeaturedModelShortcuts.tsx",
    );

    expect(presentation).toContain("artworkSrc?: string");
    expect(presentation).toContain("getModelPresentation");
    expect(presentation).toContain("/app/image?model=${encodeURIComponent(model.id)}");
    expect(card).toContain("getModelPresentation(model)");
    expect(selector).toContain('<ModelCard');
    expect(selector).toContain('variant="selector"');
    expect(selector).toContain("getModelPresentation(selectedModel)");
    expect(selector).toContain("src={selectedModelPresentation?.artworkSrc}");
    expect(selector).not.toMatch(
      /styles\.(?:option|optionSelected|optionIcon|optionCopy|optionTitle|optionDescription)\b/,
    );
    expect(workspaceSelector).toContain("<ModelSelector");
    expect(shortcuts).toContain("getModelPresentation(model)");
    expect(shortcuts).not.toContain("artworkByModelId");
  });
});
