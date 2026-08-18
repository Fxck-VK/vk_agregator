import { contentGraphSchema } from "../domain/schemas";
import type { ArticleContent, ContentGraph, ModelContent, ToolContent } from "../domain/types";

type ContentEntity = ModelContent | ToolContent | ArticleContent;
type ContentKind = ContentEntity["kind"];
type EntityLocation = {
  entity: ContentEntity;
  location: string;
};

const collectionByKind = {
  model: "models",
  tool: "tools",
  article: "articles",
} as const;

function buildEntityIndex(graph: ContentGraph): Map<string, EntityLocation> {
  const index = new Map<string, EntityLocation>();

  for (const kind of ["model", "tool", "article"] as const) {
    const collection = graph[collectionByKind[kind]] as ContentEntity[];
    collection.forEach((entity, position) => {
      const location = `${collectionByKind[kind]}[${position}]`;
      if (index.has(entity.id)) {
        throw new Error(`Duplicate content ID "${entity.id}" at ${location}.`);
      }
      index.set(entity.id, { entity, location });
    });
  }

  return index;
}

function assertUniqueLocalizedSlugs(graph: ContentGraph) {
  for (const kind of ["model", "tool", "article"] as const) {
    const collection = graph[collectionByKind[kind]] as ContentEntity[];
    for (const locale of ["ru", "en"] as const) {
      const seen = new Map<string, string>();

      for (const entity of collection) {
        const translation = entity.translations[locale];
        if (!translation) continue;

        for (const slug of [translation.slug, ...translation.legacySlugs]) {
          const previous = seen.get(slug);
          if (previous) {
            throw new Error(
              `Duplicate ${kind} slug "${slug}" for locale "${locale}" between "${previous}" and "${entity.id}".`,
            );
          }
          seen.set(slug, entity.id);
        }
      }
    }
  }
}

function assertReferences(
  source: ContentEntity,
  field: string,
  ids: readonly string[],
  expectedKind: ContentKind,
  index: Map<string, EntityLocation>,
) {
  for (const id of ids) {
    const target = index.get(id);
    if (!target) {
      throw new Error(`${source.kind} "${source.id}" ${field} references missing ${expectedKind} "${id}".`);
    }
    if (target.entity.kind !== expectedKind) {
      throw new Error(
        `${source.kind} "${source.id}" ${field} expected ${expectedKind} but found ${target.entity.kind} "${id}".`,
      );
    }
  }
}

function assertRelationships(graph: ContentGraph, index: Map<string, EntityLocation>) {
  for (const model of graph.models) {
    assertReferences(model, "relatedToolIds", model.relatedToolIds, "tool", index);
    assertReferences(model, "relatedArticleIds", model.relatedArticleIds, "article", index);
  }

  for (const tool of graph.tools) {
    assertReferences(tool, "relatedModelIds", tool.relatedModelIds, "model", index);
    assertReferences(tool, "relatedArticleIds", tool.relatedArticleIds, "article", index);
  }

  for (const article of graph.articles) {
    assertReferences(article, "relatedModelIds", article.relatedModelIds, "model", index);
    assertReferences(article, "relatedToolIds", article.relatedToolIds, "tool", index);
  }
}

export function validateContentGraph(input: unknown): ContentGraph {
  const graph = contentGraphSchema.parse(input);
  const index = buildEntityIndex(graph);
  assertUniqueLocalizedSlugs(graph);
  assertRelationships(graph, index);
  return graph;
}
