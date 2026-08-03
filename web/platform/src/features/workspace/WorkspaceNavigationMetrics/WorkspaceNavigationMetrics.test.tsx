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

  it("does not collect metrics on a production hostname", () => {
    vi.stubGlobal("location", new URL("https://neiirohub.ru/app"));

    recordWorkspaceDataLoad({ type: "data", target: "files", source: "network", durationMs: 24 });

    expect(getMetrics()).toEqual([]);
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
