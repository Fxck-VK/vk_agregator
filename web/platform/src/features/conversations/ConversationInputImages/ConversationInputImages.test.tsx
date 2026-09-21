import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { ConversationInputImages } from "./ConversationInputImages";

vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: vi.fn() }));
const id = "f0000000-0000-4000-8000-000000000001";
const createURL = vi.fn(() => "blob:local-photo");
const revokeURL = vi.fn();
const imageResponse = () => new Response(new Uint8Array([137, 80, 78, 71]), { headers: { "Content-Type": "image/png" } });

describe("conversation input images", () => {
  beforeEach(() => {
    vi.mocked(webBrowserFetch).mockReset();
    createURL.mockClear(); revokeURL.mockClear();
    vi.stubGlobal("URL", class extends URL {
      static createObjectURL = createURL;
      static revokeObjectURL = revokeURL;
    });
  });
  afterEach(() => vi.unstubAllGlobals());

  it("loads through the authenticated transport and releases its own preview on unmount", async () => {
    vi.mocked(webBrowserFetch).mockResolvedValue(imageResponse());
    const view = render(<ConversationInputImages ids={[id]} />);
    const image = await screen.findByRole("img", { name: "Прикреплённое изображение" });
    expect(image).toHaveAttribute("src", "blob:local-photo");
    expect(screen.getByRole("button", { name: "Просмотр файла: Прикреплённое изображение" })).toBeEnabled();
    expect(webBrowserFetch).toHaveBeenCalledWith(`/web/v1/input-artifacts/${id}`, { signal: expect.any(AbortSignal) });
    const signal = vi.mocked(webBrowserFetch).mock.calls[0][1]!.signal!;
    view.unmount();
    expect(signal.aborted).toBe(true);
    expect(revokeURL).toHaveBeenCalledWith("blob:local-photo");
  });

  it("opens the selected photo in the shared gallery without metadata and returns focus on close", async () => {
    vi.mocked(webBrowserFetch).mockImplementation(async () => imageResponse());
    createURL.mockReturnValueOnce("blob:first").mockReturnValueOnce("blob:second");
    render(<ConversationInputImages ids={[id, "second-id"]} />);
    await screen.findAllByRole("img", { name: "Прикреплённое изображение" });
    const triggers = await screen.findAllByRole("button", { name: "Просмотр файла: Прикреплённое изображение" });
    expect(triggers).toHaveLength(2);
    triggers[1].focus();
    fireEvent.click(triggers[1]);
    const dialog = screen.getByRole("dialog", { name: "Просмотр файла" });
    expect(within(dialog).queryByRole("complementary")).not.toBeInTheDocument();
    expect(within(screen.getByTestId("attachment-preview-preview")).getByRole("img")).toHaveAttribute("src", "blob:second");
    fireEvent.keyDown(dialog, { key: "ArrowLeft" });
    expect(within(screen.getByTestId("attachment-preview-preview")).getByRole("img")).toHaveAttribute("src", "blob:first");
    fireEvent.keyDown(window, { key: "Escape" });
    fireEvent.animationEnd(screen.getByTestId("attachment-preview-backdrop"));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(triggers[1]).toHaveFocus();
    expect(revokeURL).not.toHaveBeenCalled();
    expect(webBrowserFetch).toHaveBeenCalledTimes(2);
  });

  it.each(["HTTP failure", "unsupported content", "undecodable image"])("offers an icon retry for %s instead of a broken image", async failure => {
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(failure === "HTTP failure"
      ? new Response(null, { status: 404 })
      : failure === "unsupported content" ? new Response("unavailable", { headers: { "Content-Type": "text/html" } }) : imageResponse());
    const view = render(<ConversationInputImages ids={[id]} />);
    if (failure === "undecodable image") fireEvent.error(await screen.findByRole("img"));
    const retry = await screen.findByRole("button", { name: "Повторить загрузку прикреплённого изображения" });
    expect(screen.queryByRole("img", { name: "Прикреплённое изображение" })).not.toBeInTheDocument();
    vi.mocked(webBrowserFetch).mockResolvedValueOnce(imageResponse());
    fireEvent.click(retry);
    expect(await screen.findByRole("img", { name: "Прикреплённое изображение" })).toHaveAttribute("src", "blob:local-photo");
    expect(webBrowserFetch).toHaveBeenCalledTimes(2);
    view.unmount();
  });

  it("ignores a late image response after unmount", async () => {
    let resolve!: (response: Response) => void;
    vi.mocked(webBrowserFetch).mockReturnValue(new Promise(done => { resolve = done; }));
    const view = render(<ConversationInputImages ids={[id]} />);
    await waitFor(() => expect(webBrowserFetch).toHaveBeenCalledOnce());
    view.unmount();
    await act(async () => resolve(imageResponse()));
    expect(createURL).not.toHaveBeenCalled();
  });
});
