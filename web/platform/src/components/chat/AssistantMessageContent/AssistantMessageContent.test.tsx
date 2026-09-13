import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { AssistantMessageContent } from "./AssistantMessageContent";

describe("AssistantMessageContent", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders an assistant answer as a structured document", () => {
    const { container } = render(
      <AssistantMessageContent
        markdown={`## Что это значит

Есть два этапа:

1. **Первый пункт**
2. Второй пункт

> Важное примечание

[Документация](https://example.com)`}
      />,
    );

    expect(screen.getByRole("heading", { level: 2, name: "Что это значит" })).toBeVisible();
    const list = screen.getByRole("list");
    expect(within(list).getAllByRole("listitem")).toHaveLength(2);
    expect(within(list).getByText("Первый пункт").tagName).toBe("STRONG");
    expect(container.querySelector("blockquote")).toHaveTextContent("Важное примечание");
    expect(screen.getByRole("link", { name: "Документация" })).toHaveAttribute("target", "_blank");
    expect(screen.getByRole("link", { name: "Документация" })).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("removes raw HTML from untrusted model output", () => {
    const { container } = render(
      <AssistantMessageContent markdown={'Безопасный текст<script>alert("xss")</script>'} />,
    );

    expect(screen.getByText("Безопасный текст")).toBeVisible();
    expect(container.querySelector("script")).toBeNull();
    expect(screen.queryByText(/alert/)).toBeNull();
  });

  it("renders a generated image from the internal artifact route with a download link", () => {
    const artifactPath = "/web/v1/image-artifacts/40000000-0000-4000-8000-000000000001";
    render(
      <AssistantMessageContent
        markdown={`Готово.\n\n![Белый бумажный журавль на облаке](${artifactPath})\n\n[Скачать изображение](${artifactPath})`}
      />,
    );

    expect(screen.getByRole("img", { name: "Белый бумажный журавль на облаке" })).toHaveAttribute("src", artifactPath);
    const download = screen.getByRole("link", { name: "Скачать изображение" });
    expect(download).toHaveAttribute("href", artifactPath);
    expect(download).toHaveAttribute("download");
  });

  it.each([
    "https://example.com/tracker.png",
    "//example.com/tracker.png",
    "data:image/png;base64,AAAA",
    "javascript:alert%281%29",
    "/web/v1/me",
    "/web/v1/image-artifacts/40000000-0000-4000-8000-000000000001?redirect=https://example.com",
  ])("does not load an image outside the artifact route: %s", (imagePath) => {
    const { container } = render(<AssistantMessageContent markdown={`![Изображение](${imagePath})`} />);

    expect(container.querySelector("img")).toBeNull();
  });
});
