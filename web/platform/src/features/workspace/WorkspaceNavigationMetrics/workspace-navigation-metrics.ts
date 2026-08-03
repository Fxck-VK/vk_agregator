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
const startedAtByTarget = new Map<WorkspaceMetricTarget, number>();
const fixedTargets: Record<string, Exclude<WorkspaceMetricTarget, "conversation">> = {
  "/app": "workspace",
  "/app/chats": "chats",
  "/app/files": "files",
  "/app/models": "models",
  "/app/inspiration": "inspiration",
};

function getMetricsWindow() {
  if (typeof window === "undefined" || !metricsHosts.has(globalThis.location.hostname)) return undefined;

  return window;
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

export function beginWorkspaceNavigation(pathname: string): void {
  const target = getTarget(pathname);

  if (!target || !getMetricsWindow()) return;

  startedAtByTarget.set(target, performance.now());
}

export function completeWorkspaceNavigation(pathname: string): void {
  const target = getTarget(pathname);
  const startedAt = target ? startedAtByTarget.get(target) : undefined;

  if (!target || startedAt === undefined || !getMetricsWindow()) return;

  startedAtByTarget.delete(target);
  const durationMs = toDurationMs(performance.now() - startedAt);

  if (durationMs === undefined) return;

  appendMetric({ type: "navigation", target, durationMs });
}

export function recordWorkspaceDataLoad(metric: Extract<WorkspaceMetric, { type: "data" }>): void {
  if (!getMetricsWindow() || (metric.target !== "files" && metric.target !== "conversation") || (metric.source !== "cache" && metric.source !== "network")) return;

  const durationMs = toDurationMs(metric.durationMs);

  if (durationMs === undefined) return;

  appendMetric({ type: "data", target: metric.target, source: metric.source, durationMs });
}
