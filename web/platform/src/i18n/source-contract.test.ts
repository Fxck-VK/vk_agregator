import { readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";
import ts from "typescript";
import { expect, it } from "vitest";

function files(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap(entry => entry.isDirectory()
    ? files(`${directory}/${entry.name}`) : [`${directory}/${entry.name}`]);
}
const root = resolve(process.cwd(), "src").replaceAll("\\", "/");
const exempt = /\/(?:i18n|test|content\/data)\/|\.test\.|\.preview\.|(?:local-)?workspace-preview\.ts$|-test-fixtures\.ts$/;
const uiAttributes = new Set(["aria-label", "alt", "title", "placeholder", "label", "description"]);
// Product/provider names and format abbreviations are language independent.
const invariantCopy = /^(?:NeiroHub|NH|ID|Nano Banana 2|Nano Banana Pro|Seedream 4\.5|VK|Google|GitHub|Telegram|ChatGPT|AI|GPT Image 2|Lite|Start\+|Pro|Ultima|Elite|Enterprise|PNG|JPEG|WebP|MP4|SVG|PDF|HD|FPS|Email|[0-9\s.,:+/%×−–—-]+)$/;

it("keeps UI copy in dictionaries and prevents direct Russian imports or fixed locale formatters", () => {
  const violations: string[] = [];
  for (const file of files(root).filter(file => /\.tsx?$/.test(file) && !exempt.test(file))) {
    const text = readFileSync(file, "utf8");
    const source = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true);
    const report = (node: ts.Node, reason: string) => violations.push(`${file.slice(root.length + 1)}:${source.getLineAndCharacterOfPosition(node.getStart(source)).line + 1} ${reason}`);
    function visit(node: ts.Node) {
      if (ts.isImportDeclaration(node) && /i18n\/(?:public\/)?ru$/.test((node.moduleSpecifier as ts.StringLiteral).text)) report(node, "Import the locale-aware dictionary/provider");
      if (ts.isImportDeclaration(node) && /^next\/(?:link|navigation)$/.test((node.moduleSpecifier as ts.StringLiteral).text)) report(node, "Use the locale-aware Link/navigation adapter");
      if (ts.isStringLiteralLike(node) || ts.isJsxText(node)) {
        const value = node.text.trim();
        if (/[А-Яа-яЁё]/.test(value)) report(node, "Inline Russian copy");
        else if (/[A-Za-z]/.test(value) && !invariantCopy.test(value) && (ts.isJsxText(node) || ts.isJsxAttribute(node.parent) && uiAttributes.has(node.parent.name.getText(source)))) report(node, `Inline UI copy: ${value}`);
      }
      if (ts.isNewExpression(node) && /^Intl\.(NumberFormat|DateTimeFormat|PluralRules)$/.test(node.expression.getText(source)) && node.arguments?.[0] && ts.isStringLiteral(node.arguments[0])) report(node, "Pass the selected locale to Intl");
      ts.forEachChild(node, visit);
    }
    visit(source);
  }
  expect(violations).toEqual([]);
});
