/* eslint-disable @next/next/no-html-link-for-pages -- Native link fixtures and router mocks are intentional in these tests. */
import { act, cleanup, fireEvent, render } from "@testing-library/react";
import { usePathname } from "next/navigation";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({
  usePathname: vi.fn(() => "/app"),
}));

import {
  beginWorkspaceNavigation,
  completeWorkspaceNavigation,
  recordWorkspaceDataLoad,
  recordWorkspaceRead,
  measureWorkspaceRead,
  type WorkspaceMetric,
} from "./workspace-navigation-metrics";
import { WorkspaceNavigationMetrics } from "./WorkspaceNavigationMetrics";

const metricsKey = "__NEIROHUB_WORKSPACE_METRICS__";
type MetricsWindow = Window & { [metricsKey]?: WorkspaceMetric[] };

function getMetrics() {
  return (window as MetricsWindow)[metricsKey] ?? [];
}

describe("workspace navigation metrics", () => {
  afterEach(() => {
    cleanup();
    delete (window as MetricsWindow)[metricsKey];
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("projects read diagnostics to safe finite fields and rejects unknown categories", () => {
    vi.stubGlobal("location", new URL("http://localhost/app"));
    const input = { target: "models" as const, source: "server" as const, outcome: "success" as const, durationMs: 12.8, payload: "private", url: "/private/id", accountId: "private" };
    recordWorkspaceRead(input);
    for (const invalid of [
      { ...input, target: "private" }, { ...input, source: "private" }, { ...input, outcome: "private" },
      { ...input, durationMs: Infinity }, { ...input, durationMs: NaN }, { ...input, durationMs: "12" },
    ]) recordWorkspaceRead(invalid as unknown as Parameters<typeof recordWorkspaceRead>[0]);
    expect(getMetrics()).toEqual([{ type: "read", target: "models", source: "server", outcome: "success", durationMs: 13 }]);
  });

  it("measures the complete asynchronous read once and returns its original result", async () => {
    vi.stubGlobal("location", new URL("http://localhost/app"));
    const now = vi.spyOn(performance, "now").mockReturnValue(10);
    let finish!: (value: { privateValue: string }) => void;
    const result = { privateValue: "not diagnostic data" };
    const read = vi.fn(() => new Promise<typeof result>(resolve => { finish = resolve; }));
    const pending = measureWorkspaceRead("balance", "network", read);
    expect(read).toHaveBeenCalledOnce();
    expect(getMetrics()).toEqual([]);
    now.mockReturnValue(42.6);
    finish(result);
    expect(await pending).toBe(result);
    expect(getMetrics()).toEqual([{ type: "read", target: "balance", source: "network", outcome: "success", durationMs: 33 }]);
  });

  it("records a failed read without capturing its error and rethrows the same error", async () => {
    vi.stubGlobal("location", new URL("https://dev-web.epifera.com/app"));
    vi.spyOn(performance, "now").mockReturnValueOnce(10).mockReturnValueOnce(31);
    const error = new Error("private error details");
    await expect(measureWorkspaceRead("conversations", "network", async () => { throw error; })).rejects.toBe(error);
    expect(getMetrics()).toEqual([{ type: "read", target: "conversations", source: "network", outcome: "failure", durationMs: 21 }]);
  });

  it("does not record or time reads on production while preserving the operation", async () => {
    vi.stubGlobal("location", new URL("https://epifera.com/app"));
    const now = vi.spyOn(performance, "now");
    recordWorkspaceRead({ target: "identity", source: "server", outcome: "success", durationMs: 30 });
    const read = vi.fn(async () => 7);
    expect(await measureWorkspaceRead("models", "cache", read)).toBe(7);
    expect(read).toHaveBeenCalledOnce();
    expect(now).not.toHaveBeenCalled();
    expect(getMetrics()).toEqual([]);
  });

  it("shares the fifty-entry limit for navigation, data and read diagnostics", () => {
    vi.stubGlobal("location", new URL("http://localhost/app"));
    recordWorkspaceDataLoad({ type: "data", target: "files", source: "cache", durationMs: 0 });
    for (let durationMs = 1; durationMs <= 51; durationMs++) recordWorkspaceRead({ target: "models", source: "cache", outcome: "success", durationMs });
    expect(getMetrics()).toHaveLength(50);
    expect(getMetrics()[0]).toEqual({ type: "read", target: "models", source: "cache", outcome: "success", durationMs: 2 });
  });

  it("records the delivered identity timing once across renders", () => {
    vi.stubGlobal("location", new URL("http://localhost/app"));
    const rendered = render(<WorkspaceNavigationMetrics identityDurationMs={18.6} />);
    rendered.rerender(<WorkspaceNavigationMetrics identityDurationMs={18.6} />);
    expect(getMetrics()).toEqual([{ type: "read", target: "identity", source: "server", outcome: "success", durationMs: 19 }]);
  });

  it("records unavailable identity reads as failure without adding visible UI", () => {
    vi.stubGlobal("location", new URL("http://localhost/app"));
    const { container } = render(<WorkspaceNavigationMetrics identityDurationMs={44.6} identityOutcome="failure" />);
    expect(container).toBeEmptyDOMElement();
    expect(getMetrics()).toEqual([{ type: "read", target: "identity", source: "server", outcome: "failure", durationMs: 45 }]);
  });

  it("does not collect metrics on a production hostname", () => {
    vi.stubGlobal("location", new URL("https://epifera.com/app"));

    recordWorkspaceDataLoad({ type: "data", target: "files", source: "network", durationMs: 24 });

    expect(getMetrics()).toEqual([]);
  });

  it("does not install a document click listener on a production hostname", async () => {
    vi.stubGlobal("location", new URL("https://epifera.com/app"));
    const addEventListener = vi.spyOn(document, "addEventListener");

    render(<WorkspaceNavigationMetrics />);
    await act(async () => {});

    expect(addEventListener.mock.calls.filter(([type, , options]) => type === "click" && options === true)).toHaveLength(0);
  });

  it("keeps data metrics limited to their category, source, and integer duration", () => {
    vi.stubGlobal("location", new URL("http://localhost/app"));

    recordWorkspaceDataLoad({ type: "data", target: "files", source: "cache", durationMs: 24.6 });

    expect(getMetrics()).toEqual([
      { type: "data", target: "files", source: "cache", durationMs: 25 },
    ]);
    expect(getMetrics()[0]).not.toHaveProperty("pathname");
    expect(getMetrics()[0]).not.toHaveProperty("href");
  });

  it("retains only the most recent fifty metrics", () => {
    vi.stubGlobal("location", new URL("http://localhost/app"));

    for (let index = 0; index < 51; index += 1) {
      recordWorkspaceDataLoad({ type: "data", target: "files", source: "network", durationMs: index });
    }

    expect(getMetrics()).toHaveLength(50);
    expect(getMetrics()[0]).toEqual({ type: "data", target: "files", source: "network", durationMs: 1 });
    expect(getMetrics()[49]).toEqual({ type: "data", target: "files", source: "network", durationMs: 50 });
  });

  it("categorises a conversation path without retaining its identifier", () => {
    vi.stubGlobal("location", new URL("http://localhost/app"));
    vi.spyOn(performance, "now").mockReturnValueOnce(20).mockReturnValueOnce(37.8);

    beginWorkspaceNavigation("/app/chat/private-conversation-id");
    completeWorkspaceNavigation("/app/chat/private-conversation-id");

    expect(getMetrics()).toEqual([
      { type: "navigation", target: "conversation", durationMs: 18 },
    ]);
    expect(JSON.stringify(getMetrics())).not.toContain("private-conversation-id");
  });

  it("records a normal click to a changed fixed workspace route when the pathname updates", async () => {
    let pathname = "/app";
    vi.mocked(usePathname).mockImplementation(() => pathname);
    vi.stubGlobal("location", new URL(`${window.location.origin}/app`));
    const now = vi.spyOn(performance, "now").mockReturnValue(100);
    const rendered = render(
      <>
        <a href="/app/files" onClick={(event) => event.preventDefault()}>Files</a>
        <WorkspaceNavigationMetrics />
      </>,
    );

    await act(async () => {});
    fireEvent.click(document.querySelector<HTMLAnchorElement>("a[href='/app/files']")!);
    now.mockReturnValue(143.2);
    pathname = "/app/files";
    rendered.rerender(
      <>
        <a href="/app/files" onClick={(event) => event.preventDefault()}>Files</a>
        <WorkspaceNavigationMetrics />
      </>,
    );

    expect(getMetrics()).toEqual([
      { type: "navigation", target: "files", durationMs: 43 },
    ]);
  });

  it.each(["_parent", "_top", "workspace-frame"])("does not measure navigation from a %s target", async (target) => {
    let pathname = "/app";
    vi.mocked(usePathname).mockImplementation(() => pathname);
    vi.stubGlobal("location", new URL(`${window.location.origin}/app`));
    const now = vi.spyOn(performance, "now").mockReturnValue(100);
    const rendered = render(
      <>
        <a href="/app/files" onClick={(event) => event.preventDefault()} target={target}>Files</a>
        <WorkspaceNavigationMetrics />
      </>,
    );

    await act(async () => {});
    fireEvent.click(document.querySelector<HTMLAnchorElement>("a[href='/app/files']")!);
    now.mockReturnValue(143.2);
    pathname = "/app/files";
    rendered.rerender(
      <>
        <a href="/app/files" onClick={(event) => event.preventDefault()} target={target}>Files</a>
        <WorkspaceNavigationMetrics />
      </>,
    );

    expect(getMetrics()).toEqual([]);
  });

  it("ignores a stale pending navigation duration", () => {
    vi.stubGlobal("location", new URL("http://localhost/app"));
    const now = vi.spyOn(performance, "now").mockReturnValue(100);

    beginWorkspaceNavigation("/app/files");
    now.mockReturnValue(10_101);
    completeWorkspaceNavigation("/app/files");

    expect(getMetrics()).toEqual([]);
  });

  it("ignores modified, non-primary, external, and unchanged workspace link clicks", () => {
    vi.stubGlobal("location", new URL("http://localhost/app"));
    render(
      <>
        <a href="/app" onClick={(event) => event.preventDefault()}>Current</a>
        <a href="/app/files" onClick={(event) => event.preventDefault()}>Files</a>
        <a href="https://example.test/app/files" onClick={(event) => event.preventDefault()}>External</a>
        <WorkspaceNavigationMetrics />
      </>,
    );

    fireEvent.click(document.querySelector<HTMLAnchorElement>("a[href='/app']")!);
    fireEvent.click(document.querySelector<HTMLAnchorElement>("a[href='/app/files']")!, { ctrlKey: true });
    fireEvent.click(document.querySelector<HTMLAnchorElement>("a[href='https://example.test/app/files']")!);
    fireEvent.click(document.querySelector<HTMLAnchorElement>("a[href='/app/files']")!, { button: 1 });

    expect(getMetrics()).toEqual([]);
  });
});
