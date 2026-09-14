import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { ru } from "@/i18n/ru";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));
import { webBrowserFetch } from "@/lib/web-api/browser";
import { ModelsCatalog } from "./ModelsCatalog";

it.each([Response.json({}, { status: 503 }), Response.json({ items: [] })])("shows load failure for an unavailable or invalid shared catalog", async response => {
  vi.mocked(webBrowserFetch).mockResolvedValueOnce(response);
  render(<ModelsCatalog />);
  expect(await screen.findByRole("alert")).toHaveTextContent(ru.modelsCatalog.loadFailure);
  expect(screen.queryByText(ru.modelsCatalog.empty)).not.toBeInTheDocument();
});
