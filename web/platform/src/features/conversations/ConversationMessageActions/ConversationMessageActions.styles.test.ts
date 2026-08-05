import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(
    process.cwd(),
    "src/features/conversations/ConversationMessageActions/ConversationMessageActions.module.css",
  ),
  "utf8",
);

describe("ConversationMessageActions styles", () => {
  it("uses one branded tooltip below every message action", () => {
    expect(stylesheet).toMatch(/\.action\[data-tooltip\]::after\s*\{/);
    expect(stylesheet).toMatch(
      /\.action\[data-tooltip\]::after\s*\{[^}]*inset-block-start:\s*calc\(100% \+ var\(--space-2\)\);/s,
    );
    expect(stylesheet).not.toMatch(
      /\.action\[data-tooltip\]::after\s*\{[^}]*inset-block-end:\s*calc\(100% \+ var\(--space-2\)\);/s,
    );
    expect(stylesheet).toMatch(/\.action\[data-tooltip\]:hover::after/);
    expect(stylesheet).toMatch(/\.action\[data-tooltip\]:focus-visible::after/);
  });
});
