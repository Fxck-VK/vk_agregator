import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ push: vi.fn() })),
}));

import { ru } from "@/i18n/ru";

import { WorkspaceHome } from "./WorkspaceHome";

describe("WorkspaceHome", () => {
  it("uses a normal chat start prompt and keeps image generation on its explicit route", () => {
    const markup = renderToStaticMarkup(<WorkspaceHome />);

    expect(markup).toContain(ru.workspace.startTitle);
    expect(markup).toContain(ru.workspace.promptSupport);
    expect(markup).toContain('href="/app/image"');
    expect(markup).toContain('href="/app/models"');
    expect(markup).not.toContain("image-generation-title");
    expect(markup).not.toContain("image-job-history-title");
  });
});
