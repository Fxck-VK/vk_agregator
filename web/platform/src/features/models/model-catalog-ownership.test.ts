// Architecture guard: UI may reference IDs for curation, but cannot own model
// records, prices or requests to the retired modality-specific catalogs.
import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";
import ts from "typescript";
import { expect, it } from "vitest";

function sources(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap(entry => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) return sources(path);
    return /\.tsx?$/.test(entry.name) && !/\.test\.|test-fixtures/.test(entry.name) ? [path] : [];
  });
}

it("keeps model definitions and pricing out of functional UI modules", () => {
  const root = join(process.cwd(), "src");
  const violations: string[] = [];
  const roots = ["features/models", "features/conversations", "features/workspace", "features/image-generation", "features/files", "components/layout/WorkspaceHeader"];
  for (const file of roots.flatMap(directory => sources(join(root, directory)))) {
    const source = ts.createSourceFile(file, readFileSync(file, "utf8"), ts.ScriptTarget.Latest, true);
    const report = (node: ts.Node, reason: string) => {
      violations.push(`${relative(root, file)}:${source.getLineAndCharacterOfPosition(node.getStart()).line + 1} ${reason}`);
    };
    const inspect = (node: ts.Node) => {
      if (ts.isStringLiteralLike(node) && /\/web\/v1\/(image|chat|video)-models\b/.test(node.text)) report(node, "use the shared catalog loader");
      if (ts.isObjectLiteralExpression(node)) {
        const properties = node.properties.filter(ts.isPropertyAssignment);
        const get = (name: string) => properties.find(property => property.name.getText(source).replace(/["']/g, "") === name)?.initializer;
        const id = get("id");
        if (id && ts.isStringLiteralLike(id) && get("name") && ["quality_options", "estimate_credits", "price_by_option", "categories"].some(name => get(name))) {
          report(node, "inline model record; keep only curated IDs");
        }
        for (const key of ["price_by_quality", "price_by_variant", "price_by_option"]) {
          const value = get(key);
          if (value && ts.isObjectLiteralExpression(value) && value.properties.length > 0) report(value, "inline model prices");
        }
      }
      ts.forEachChild(node, inspect);
    };
    inspect(source);
  }
  expect(violations).toEqual([]);
});
