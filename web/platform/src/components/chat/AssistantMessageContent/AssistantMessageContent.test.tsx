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
});
