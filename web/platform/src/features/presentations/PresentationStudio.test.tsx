import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { PresentationStudio } from "./PresentationStudio";

const client = vi.hoisted(() => ({ read: vi.fn(), write: vi.fn(), search: "" }));
vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: client.read, webBrowserMutation: client.write }));
vi.mock("@/i18n/navigation", () => ({
  useRouter: () => ({}),
  useSearchParams: () => new URLSearchParams(client.search),
}));
vi.mock("@/i18n/Link", () => ({
  default: ({ children, href }: { children: string; href: string }) => <a href={href}>{children}</a>,
}));

const conversation = "12345678-0000-4000-8000-012345678901";
const job = "12345678-0000-4000-8000-012345678902";

afterEach(() => { cleanup(); vi.resetAllMocks(); client.search = ""; });

it("resumes the exact URL job when browser Back restores the new presentation view", async () => {
  client.search = `run=${conversation}&job=${job}`;
  client.read.mockResolvedValue(Response.json({ job_id: job, status: "provider_processing", prompt: "" }));
  render(<PresentationStudio accountId="account" local={false} />);
  await waitFor(() => expect(screen.getByRole("status")).toHaveTextContent("Модель пишет слайды"));
  expect(client.read).toHaveBeenCalledWith(`/web/v1/presentations/${conversation}?job_id=${job}`, expect.any(Object));
  expect(client.write).not.toHaveBeenCalled();
});
