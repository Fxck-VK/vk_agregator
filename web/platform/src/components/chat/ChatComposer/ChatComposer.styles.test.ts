import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/chat/ChatComposer/ChatComposer.module.css"),
  "utf8",
);
const inputControlStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/InputControlChip/InputControlChip.module.css"),
  "utf8",
);
const mediaMenuStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/chat/ChatMediaMenu/ChatMediaMenu.module.css"),
  "utf8",
);
const templatePickerStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.module.css"),
  "utf8",
);
const aspectRatioStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector.module.css"),
  "utf8",
);
const qualityStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/image-generation/ImageQualitySelector/ImageQualitySelector.module.css"),
  "utf8",
);
const outputCountStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/image-generation/ImageOutputCountSelector/ImageOutputCountSelector.module.css"),
  "utf8",
);
const submitButtonStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/chat/ChatSubmitButton/ChatSubmitButton.module.css"),
  "utf8",
);
const composerSource = readFileSync(
  resolve(process.cwd(), "src/components/chat/ChatComposer/ChatComposer.tsx"),
  "utf8",
);

function rule(source: string, selector: string) {
  return source.match(new RegExp(`${selector}\\s*\\{([^}]*)\\}`, "s"))?.[1] ?? "";
}

describe("ChatComposer landing styles", () => {
  it("places the controls below the one-line input", () => {
    expect(stylesheet).toMatch(
      /\.hero\s*\{[^}]*grid-template-areas:\s*"field" "controls" "attachment";/s,
    );
    expect(stylesheet).toMatch(
      /\.newChat\s*\{[^}]*grid-template-areas:\s*"field" "controls" "attachment";/s,
    );
    expect(stylesheet).toMatch(/\.field\s*\{[^}]*grid-area:\s*field;/s);
    expect(stylesheet).toMatch(/\.controls\s*\{[^}]*grid-area:\s*controls;/s);
    expect(stylesheet).toMatch(/\.attachment\s*\{[^}]*grid-area:\s*attachment;/s);
  });

  it("lets the shared text input own wrapping and vertical growth", () => {
    expect(stylesheet).not.toMatch(/\.hero textarea\s*\{/);
    expect(stylesheet).not.toMatch(/\.newChat textarea\s*\{/);
  });

  it("gives every control subcomponent the same small radius", () => {
    expect(stylesheet).toMatch(
      /\.surface \[data-ui="input-control-chip"\]\s*\{[^}]*border-radius:\s*var\(--radius-sm\);/s,
    );
    expect(rule(submitButtonStylesheet, "\\.button")).toContain(
      "border-radius: var(--radius-sm)",
    );
    expect(stylesheet).not.toContain("data-control-radius");
  });

  it("delegates the submit action to ChatSubmitButton without keeping its old markup", () => {
    expect(composerSource).toContain(
      'from "@/components/chat/ChatSubmitButton/ChatSubmitButton"',
    );
    expect(composerSource).toContain("<ChatSubmitButton");
    expect(composerSource).not.toContain("className={styles.submit}");
    expect(composerSource).not.toContain("<svg");
  });

  it("scales every control subcomponent from shared compact metrics", () => {
    const surfaceRule = rule(stylesheet, "\\.surface");
    const chipRule = rule(inputControlStylesheet, "\\.chip");
    const groupRule = rule(inputControlStylesheet, "\\.group");
    const submitRule = rule(submitButtonStylesheet, "\\.button");

    expect(surfaceRule).toContain("--input-control-size: 2.5rem");
    expect(surfaceRule).toContain("--input-control-padding-inline: var(--space-3)");
    expect(surfaceRule).toContain("--input-control-icon-size: 1.125rem");
    expect(surfaceRule).toContain("--input-control-row-gap: var(--space-2)");
    expect(chipRule).toContain("min-block-size: var(--input-control-size, 3.25rem)");
    expect(chipRule).toContain("padding-inline: var(--input-control-padding-inline, var(--space-4))");
    expect(groupRule).toContain("padding-inline: var(--input-control-group-padding-inline, 0.45rem)");
    expect(submitRule).toContain("inline-size: var(--input-control-size, 3.25rem)");
    expect(submitRule).toContain("block-size: var(--input-control-size, 3.25rem)");
    expect(rule(submitButtonStylesheet, "\\.button img")).toContain(
      "inline-size: var(--input-control-icon-size, 1.5rem)",
    );
    expect(rule(mediaMenuStylesheet, "\\.trigger img")).toContain(
      "inline-size: var(--chat-media-menu-icon-size)",
    );
    expect(rule(mediaMenuStylesheet, "\\.trigger")).toContain(
      "--chat-media-menu-icon-size: 1.4rem",
    );
    expect(rule(templatePickerStylesheet, "\\.triggerIcon")).toContain(
      "inline-size: var(--input-control-icon-size, 1.35rem)",
    );
    expect(rule(aspectRatioStylesheet, "\\.ratioIcon")).toContain(
      "inline-size: var(--input-control-icon-size, 1.5rem)",
    );
    expect(rule(qualityStylesheet, "\\.tuneIcon")).toContain(
      "inline-size: var(--input-control-icon-size, 1.25rem)",
    );
    expect(rule(qualityStylesheet, "\\.chevronIcon")).toContain(
      "inline-size: var(--input-control-chevron-size, 0.9rem)",
    );
    expect(rule(outputCountStylesheet, "\\.button")).toContain(
      "height: var(--input-control-step-size, 2.25rem)",
    );
    expect(rule(outputCountStylesheet, "\\.value")).toContain(
      "min-width: var(--input-control-value-width, 3.4rem)",
    );
  });
});
