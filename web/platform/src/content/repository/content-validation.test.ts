import { describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));

import { rawContentGraph } from "../data/content";
import type { ContentGraph } from "../domain/types";
import { validateContentGraph } from "./content-validation";

function cloneGraph(): ContentGraph {
  return structuredClone(rawContentGraph);
}

describe("content graph validation", () => {
  it("accepts the original bilingual repository seed graph", () => {
    expect(validateContentGraph(rawContentGraph)).toEqual(rawContentGraph);
  });

  it("rejects a duplicate stable ID with an actionable entity reference", () => {
    const graph = cloneGraph();
    graph.models.push(structuredClone(graph.models[0]));

    expect(() => validateContentGraph(graph)).toThrow(
      'Duplicate content ID "model-gpt-image-2" at models[1].',
    );
  });

  it("rejects duplicate current and legacy slugs within one route family and locale", () => {
    const graph = cloneGraph();
    const duplicate = structuredClone(graph.models[0]);
    duplicate.id = "model-gpt-image-2-preview";
    graph.models.push(duplicate);

    expect(() => validateContentGraph(graph)).toThrow(
      'Duplicate model slug "gpt-image-2" for locale "ru"',
    );
  });

  it("rejects a missing relationship and identifies its field", () => {
    const graph = cloneGraph();
    graph.models[0].relatedToolIds = ["tool-missing"];

    expect(() => validateContentGraph(graph)).toThrow(
      'model "model-gpt-image-2" relatedToolIds references missing tool "tool-missing".',
    );
  });

  it("rejects a relationship that points to the wrong entity kind", () => {
    const graph = cloneGraph();
    graph.models[0].relatedToolIds = ["article-image-prompt-basics"];

    expect(() => validateContentGraph(graph)).toThrow(
      'model "model-gpt-image-2" relatedToolIds expected tool but found article "article-image-prompt-basics".',
    );
  });

  it("rejects malformed records before relationship checks and returns no partial graph", () => {
    const graph = cloneGraph() as ContentGraph & { models: Array<Record<string, unknown>> };
    graph.models[0].price = 55;

    expect(() => validateContentGraph(graph)).toThrow();
  });
});
