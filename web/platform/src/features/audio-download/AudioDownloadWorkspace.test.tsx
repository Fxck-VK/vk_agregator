import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";

import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { AudioDownloadWorkspace } from "./AudioDownloadWorkspace";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn(), webBrowserMutation: vi.fn() }));

const jobId = "11111111-1111-4111-8111-111111111111";
const artifactId = "22222222-2222-4222-8222-222222222222";
const sourceURL = "https://soundcloud.com/artist/track";

beforeEach(() => {
  vi.resetAllMocks();
  sessionStorage.clear();
});

it("submits a trimmed link once, then offers the MP3 only after the owned job succeeds", async () => {
  vi.mocked(webBrowserMutation).mockResolvedValue(Response.json({ id: jobId, status: "queued" }, { status: 201 }));
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ id: jobId, status: "succeeded", artifact_id: artifactId }));
  const { unmount } = render(<AudioDownloadWorkspace />);
  fireEvent.change(screen.getByRole("textbox", { name: "Ссылка" }), { target: { value: ` ${sourceURL} ` } });
  const submit = screen.getByRole("button", { name: "Подготовить MP3" });
  fireEvent.click(submit);
  fireEvent.click(submit);
  expect(screen.queryByRole("link", { name: "Скачать MP3" })).not.toBeInTheDocument();

  const download = await screen.findByRole("link", { name: "Скачать MP3" });
  expect(download).toHaveAttribute("href", `/web/v1/audio-download-artifacts/${artifactId}`);
  expect(download).toHaveAttribute("download");
  expect(webBrowserMutation).toHaveBeenCalledTimes(1);
  expect(webBrowserMutation).toHaveBeenCalledWith("/web/v1/audio-download-jobs", expect.objectContaining({
    method: "POST", body: JSON.stringify({ url: sourceURL }),
    headers: expect.objectContaining({ "X-Idempotency-Key": expect.any(String) }),
  }));
  expect(webBrowserFetch).toHaveBeenCalledWith(`/web/v1/audio-download-jobs/${jobId}`, expect.objectContaining({ signal: expect.any(AbortSignal) }));

  unmount();
  vi.mocked(webBrowserFetch).mockResolvedValue(Response.json({ id: jobId, status: "succeeded", artifact_id: artifactId }));
  render(<AudioDownloadWorkspace />);
  expect(await screen.findByRole("link", { name: "Скачать MP3" })).toBeVisible();
  expect(webBrowserMutation).toHaveBeenCalledTimes(1);
});

it("reuses the idempotency key after an ambiguous submit failure", async () => {
  vi.mocked(webBrowserMutation).mockRejectedValueOnce(new TypeError("offline"))
    .mockResolvedValueOnce(Response.json({ id: jobId, status: "succeeded", artifact_id: artifactId }));
  render(<AudioDownloadWorkspace />);
  fireEvent.change(screen.getByRole("textbox", { name: "Ссылка" }), { target: { value: sourceURL } });
  const submit = screen.getByRole("button", { name: "Подготовить MP3" });
  fireEvent.click(submit);
  expect(await screen.findByRole("alert")).toHaveTextContent("Не удалось начать обработку");
  fireEvent.click(submit);
  expect(await screen.findByRole("link", { name: "Скачать MP3" })).toBeVisible();

  const [first, second] = vi.mocked(webBrowserMutation).mock.calls;
  expect(first).toEqual(second);
});
