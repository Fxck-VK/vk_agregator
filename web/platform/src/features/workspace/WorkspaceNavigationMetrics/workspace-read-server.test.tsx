import { Children, isValidElement, type ReactElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));
vi.mock("@/features/session/session-data", () => ({ loadWorkspaceSession: vi.fn() }));
vi.mock("@/features/models/model-catalog-server", () => ({ initialModelCatalog: vi.fn(() => ({ source: null, pending: null })) }));
vi.mock("@/features/session/local-workspace-preview", () => ({ isLocalWorkspacePreviewEnabled: vi.fn(() => false) }));

import { WorkspaceSessionContent } from "@/app/app/layout";
import { loadWorkspaceSession } from "@/features/session/session-data";
import { initialModelCatalog } from "@/features/models/model-catalog-server";
import { isLocalWorkspacePreviewEnabled } from "@/features/session/local-workspace-preview";
import { WorkspaceNavigationMetrics } from "./WorkspaceNavigationMetrics";
import { WorkspaceFrame } from "@/components/layout/WorkspaceFrame/WorkspaceFrame";

function frameDuration(element: ReactElement) {
  const wrapper = element as ReactElement<{ children: ReactNode }>;
  const frame = Children.toArray(wrapper.props.children).find(
    child => isValidElement(child) && child.type === WorkspaceFrame,
  ) as ReactElement<{ identityDurationMs?: number }> | undefined;
  expect(frame).toBeDefined();
  return frame?.props.identityDurationMs;
}

describe("server identity timing delivery", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.clearAllMocks();
    vi.unstubAllEnvs();
    vi.mocked(isLocalWorkspacePreviewEnabled).mockReturnValue(false);
  });

  function authenticatedSession() {
    vi.mocked(loadWorkspaceSession).mockResolvedValue({
      kind: "authenticated", profile: { account_id: "fixture", identity_refs: [] },
      conversations: [], balance: null, deferred: true,
    });
  }

  it("delivers only the completed identity duration before starting catalogue loading", async () => {
    vi.stubEnv("WEB_ORIGIN", "https://dev-web.epifera.com");
    authenticatedSession();
    vi.spyOn(performance, "now").mockReturnValueOnce(100).mockReturnValueOnce(133.5);
    const tree = await WorkspaceSessionContent({ children: null });
    expect(frameDuration(tree)).toBe(33.5);
    expect(loadWorkspaceSession).toHaveBeenCalledOnce();
    expect(vi.mocked(loadWorkspaceSession).mock.invocationCallOrder[0]).toBeLessThan(vi.mocked(initialModelCatalog).mock.invocationCallOrder[0]);
  });

  it("delivers failed identity timing in the unavailable shell without starting catalogue loading", async () => {
    vi.stubEnv("WEB_ORIGIN", "https://dev-web.epifera.com");
    vi.mocked(loadWorkspaceSession).mockResolvedValue({ kind: "unavailable" });
    vi.spyOn(performance, "now").mockReturnValueOnce(100).mockReturnValueOnce(144.6);
    const tree = await WorkspaceSessionContent({ children: null });
    const shell = tree as ReactElement<{ children: ReactElement[] }>;
    const recorder = shell.props.children[0];
    expect(recorder?.type).toBe(WorkspaceNavigationMetrics);
    expect(recorder?.props).toMatchObject({ identityDurationMs: expect.closeTo(44.6), identityOutcome: "failure" });
    expect(loadWorkspaceSession).toHaveBeenCalledOnce();
    expect(initialModelCatalog).not.toHaveBeenCalled();
  });

  it.each(["preview", "production"])("omits identity diagnostics for %s reads", async mode => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("WEB_ORIGIN", mode === "preview" ? "https://dev-web.epifera.com" : "https://epifera.com");
    vi.mocked(isLocalWorkspacePreviewEnabled).mockReturnValue(mode === "preview");
    authenticatedSession();
    const now = vi.spyOn(performance, "now");
    expect(frameDuration(await WorkspaceSessionContent({ children: null }))).toBeUndefined();
    expect(now).not.toHaveBeenCalled();
    expect(loadWorkspaceSession).toHaveBeenCalledOnce();
  });
});
