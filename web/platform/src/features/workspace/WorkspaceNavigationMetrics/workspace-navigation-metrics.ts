export type WorkspaceMetricTarget = "workspace" | "chats" | "files" | "models" | "inspiration" | "conversation";
export type WorkspaceMetric =
  | { type: "navigation"; target: WorkspaceMetricTarget; durationMs: number }
  | { type: "data"; target: "files" | "conversation"; source: "cache" | "network"; durationMs: number };

declare global {
  interface Window {
    __NEIROHUB_WORKSPACE_METRICS__?: WorkspaceMetric[];
  }
}

const metricsHosts = new Set(["localhost", "127.0.0.1", "dev-web.neiirohub.ru"]);
const maxPendingNavigationDurationMs = 10_000;
const fixedTargets: Record<string, Exclude<WorkspaceMetricTarget, "conversation">> = {
  "/app": "workspace",
  "/app/chats": "chats",
  "/app/files": "files",
  "/app/models": "models",
  "/app/inspiration": "inspiration",
};
type PendingWorkspaceNavigation = {
  target: WorkspaceMetricTarget;
  startedAt: number;
  timeoutId: number;
};

let pendingWorkspaceNavigation: PendingWorkspaceNavigation | undefined;

function getMetricsWindow() {
  if (typeof window === "undefined" || !metricsHosts.has(globalThis.location.hostname)) return undefined;

  return window;
}

export function isWorkspaceMetricsEnabled(): boolean {
  return getMetricsWindow() !== undefined;
}

function getTarget(pathname: string): WorkspaceMetricTarget | undefined {
  if (pathname in fixedTargets) return fixedTargets[pathname];
  if (/^\/app\/chat\/[^/]+$/.test(pathname)) return "conversation";

  return undefined;
}

function toDurationMs(value: number) {
  if (!Number.isFinite(value)) return undefined;

  return Math.max(0, Math.round(value));
}

function appendMetric(metric: WorkspaceMetric) {
  const metricsWindow = getMetricsWindow();

  if (!metricsWindow) return;

  const current = Array.isArray(metricsWindow.__NEIROHUB_WORKSPACE_METRICS__)
    ? metricsWindow.__NEIROHUB_WORKSPACE_METRICS__
    : [];
  metricsWindow.__NEIROHUB_WORKSPACE_METRICS__ = [...current, metric].slice(-50);
}

function clearPendingWorkspaceNavigation() {
  if (!pendingWorkspaceNavigation) return;

  window.clearTimeout(pendingWorkspaceNavigation.timeoutId);
  pendingWorkspaceNavigation = undefined;
}

export function beginWorkspaceNavigation(pathname: string): void {
  const target = getTarget(pathname);
  const metricsWindow = getMetricsWindow();

  if (!target || !metricsWindow) return;

  clearPendingWorkspaceNavigation();
  const startedAt = performance.now();

  if (!Number.isFinite(startedAt)) return;

  const pendingNavigation: PendingWorkspaceNavigation = { target, startedAt, timeoutId: 0 };
  pendingNavigation.timeoutId = metricsWindow.setTimeout(() => {
    if (pendingWorkspaceNavigation === pendingNavigation) pendingWorkspaceNavigation = undefined;
  }, maxPendingNavigationDurationMs);
  pendingWorkspaceNavigation = pendingNavigation;
}

export function completeWorkspaceNavigation(pathname: string): void {
  const target = getTarget(pathname);
  const pendingNavigation = pendingWorkspaceNavigation;

  clearPendingWorkspaceNavigation();

  if (!target || !pendingNavigation || pendingNavigation.target !== target || !getMetricsWindow()) return;

  const durationMs = toDurationMs(performance.now() - pendingNavigation.startedAt);

  if (durationMs === undefined || durationMs > maxPendingNavigationDurationMs) return;

  appendMetric({ type: "navigation", target, durationMs });
}

export function recordWorkspaceDataLoad(metric: Extract<WorkspaceMetric, { type: "data" }>): void {
  if (!getMetricsWindow() || (metric.target !== "files" && metric.target !== "conversation") || (metric.source !== "cache" && metric.source !== "network")) return;

  const durationMs = toDurationMs(metric.durationMs);

  if (durationMs === undefined) return;

  appendMetric({ type: "data", target: metric.target, source: metric.source, durationMs });
}
