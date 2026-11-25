import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { PresentationReturnLink } from "./PresentationReturnLink";

const transport = vi.hoisted(() => ({ read: vi.fn(), view: "dev" }));
vi.mock("@/lib/web-api/browser", () => ({ webBrowserFetch: transport.read }));
vi.mock("@/features/release/ReleaseViewProvider", () => ({ useReleaseView: () => ({ view: transport.view, allowsPath: () => true }) }));

const conversationId = "d7c979f5-24e5-4f88-924b-a592d6e5a906";
const jobId = "1315de47-581e-4e4e-b352-32ccb0d48893";
const label = "Вернуться к презентации";

afterEach(() => {
  cleanup();
  transport.read.mockReset();
  transport.view = "dev";
});

describe("presentation return in chat history", () => {
  it("checks ownership without reading the answer and reopens the exact presentation", async () => {
    transport.read.mockResolvedValue(new Response(null, { status: 204 }));
    render(<PresentationReturnLink conversationId={conversationId} jobId={jobId} />);
    expect(await screen.findByRole("link", { name: label })).toHaveAttribute(
      "href", "/ru/app/presentations?run=" + conversationId + "&job=" + jobId,
    );
    expect(transport.read).toHaveBeenCalledWith("/web/v1/presentations/" + conversationId + "?job_id=" + jobId, expect.objectContaining({ method: "HEAD", cache: "no-store" }));
  });

  it("leaves ordinary chats unchanged", async () => {
    transport.read.mockResolvedValue(new Response(null, { status: 404 }));
    render(<PresentationReturnLink conversationId={conversationId} />);
    await waitFor(() => expect(transport.read).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole("link", { name: label })).not.toBeInTheDocument();
  });

  it("does not probe presentations in PROD or with an invalid conversation ID", () => {
    transport.view = "prod";
    const result = render(<PresentationReturnLink conversationId={conversationId} />);
    transport.view = "dev";
    result.rerender(<PresentationReturnLink conversationId="invalid" />);
    expect(transport.read).not.toHaveBeenCalled();
    expect(screen.queryByRole("link", { name: label })).not.toBeInTheDocument();
  });
});
