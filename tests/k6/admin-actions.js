import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

const rawBaseURL = __ENV.K6_BASE_URL || __ENV.BASE_URL || "";
const allowProductionLiveSmoke = boolEnv("K6_ALLOW_PRODUCTION_LIVE_SMOKE", false);
assertSafeLoadTarget(rawBaseURL, "K6_BASE_URL/BASE_URL", allowProductionLiveSmoke);
const enabled = rawBaseURL !== "" || __ENV.K6_RUN === "1";
const baseURL = trimTrailingSlash(rawBaseURL || "http://127.0.0.1:8080");
const allowMutations = boolEnv("K6_ADMIN_MUTATION_SMOKE", false);
const runID = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

const adminAuthGuardOK = new Rate("admin_auth_guard_ok");
const adminReadOK = new Rate("admin_read_ok");
const adminActionGuardOK = new Rate("admin_action_guard_ok");
const adminReadDuration = new Trend("admin_read_duration");
const adminActionDuration = new Trend("admin_action_duration");

export const options = enabled
  ? {
      scenarios: {
        admin_auth_guard: {
          executor: "constant-vus",
          exec: "adminAuthGuard",
          vus: intEnv("K6_ADMIN_AUTH_VUS", 1),
          duration: __ENV.K6_ADMIN_DURATION || "30s",
        },
        admin_read_models: {
          executor: "constant-arrival-rate",
          exec: "adminReadModels",
          rate: intEnv("K6_ADMIN_READ_RATE", 1),
          timeUnit: "1s",
          duration: __ENV.K6_ADMIN_DURATION || "30s",
          preAllocatedVUs: intEnv("K6_ADMIN_PREALLOCATED_VUS", 5),
          maxVUs: intEnv("K6_ADMIN_MAX_VUS", 20),
        },
        admin_action_guards: {
          executor: "constant-vus",
          exec: "adminActionGuards",
          vus: intEnv("K6_ADMIN_ACTION_VUS", 1),
          duration: __ENV.K6_ADMIN_ACTION_DURATION || __ENV.K6_ADMIN_DURATION || "30s",
        },
      },
      thresholds: {
        http_req_failed: ["rate<0.05"],
        http_req_duration: ["p(95)<1500"],
        admin_auth_guard_ok: ["rate>0.95"],
        admin_read_ok: ["rate>0.95"],
        admin_action_guard_ok: ["rate>0.95"],
        admin_read_duration: ["p(95)<1500"],
        admin_action_duration: ["p(95)<2000"],
      },
      userAgent: "vk-ai-aggregator-k6-admin-actions/1.0",
      summaryTrendStats: ["avg", "min", "med", "p(90)", "p(95)", "p(99)", "max"],
    }
  : {
      scenarios: {
        disabled_without_target: {
          executor: "shared-iterations",
          exec: "noop",
          vus: 1,
          iterations: 1,
        },
      },
    };

export function noop() {
  console.log("k6 admin/operator scenario skipped: set K6_BASE_URL to run against a loadtest/dev API.");
}

export function adminAuthGuard() {
  const denied = http.get(`${baseURL}/admin/access/operator`, {
    tags: { surface: "admin", route: "access_without_token" },
  });
  const deniedOK = check(denied, {
    "admin access without token is denied": (r) => r.status === 401 || r.status === 403 || r.status === 404,
    "admin access without token is not 2xx": (r) => r.status < 200 || r.status >= 300,
  });
  adminAuthGuardOK.add(deniedOK);
  sleep(floatEnv("K6_ADMIN_SLEEP_SECONDS", 0));
}

export function adminReadModels() {
  const started = Date.now();
  const route = chooseReadRoute();
  const res = http.get(`${baseURL}${route.path}`, {
    headers: adminHeaders(),
    tags: { surface: "admin", route: route.name },
  });
  const ok = check(res, {
    [`${route.name} returns 2xx`]: (r) => r.status >= 200 && r.status < 300,
    [`${route.name} returns JSON-ish body`]: (r) => isJSONResponse(r),
    [`${route.name} does not leak obvious secrets`]: (r) => !containsSensitiveMarkers(r),
  });
  adminReadOK.add(ok, { route: route.name });
  adminReadDuration.add(Date.now() - started, { route: route.name });
  sleep(floatEnv("K6_ADMIN_SLEEP_SECONDS", 0));
}

export function adminActionGuards() {
  const started = Date.now();

  const dryRun = http.get(`${baseURL}/admin/retention/operator/dry-run?limit=10`, {
    headers: adminHeaders(),
    tags: { surface: "admin", route: "retention_dry_run" },
  });
  const dryRunOK = check(dryRun, {
    "retention dry-run is 2xx": (r) => r.status >= 200 && r.status < 300,
    "retention dry-run does not leak raw data": (r) => !containsSensitiveMarkers(r),
  });
  adminActionGuardOK.add(dryRunOK, { route: "retention_dry_run" });

  if (allowMutations) {
    const cleanup = http.post(`${baseURL}/admin/retention/operator/run-cleanup`, "{}", {
      headers: {
        ...adminHeaders(),
        "Content-Type": "application/json",
        "X-Idempotency-Key": `k6.admin.cleanup.${runID}.${__VU}.${__ITER}`,
      },
      tags: { surface: "admin", route: "retention_cleanup" },
    });
    const cleanupOK = check(cleanup, {
      "retention cleanup guarded action is controlled": (r) => r.status >= 200 && r.status < 500,
      "retention cleanup does not leak raw data": (r) => !containsSensitiveMarkers(r),
    });
    adminActionGuardOK.add(cleanupOK, { route: "retention_cleanup" });
  }

  adminActionDuration.add(Date.now() - started);
  sleep(floatEnv("K6_ADMIN_SLEEP_SECONDS", 0));
}

function chooseReadRoute() {
  const routes = [
    { name: "operator_access", path: "/admin/access/operator" },
    { name: "jobs_list", path: "/admin/jobs/operator?limit=20" },
    { name: "queue_summary", path: "/admin/jobs/queue" },
    { name: "dlq_list", path: "/admin/jobs/dlq?limit=20" },
    { name: "providers", path: "/admin/providers/operator" },
    { name: "pricing", path: "/admin/pricing/operator" },
    { name: "media_safety", path: "/admin/media-safety/operator" },
    { name: "config_health", path: "/admin/config-health/operator" },
    { name: "retention_status", path: "/admin/retention/operator/status" },
    { name: "analytics_status", path: "/admin/analytics/operator/status" },
    { name: "hot_rows", path: "/admin/data/operator/hot-rows" },
    { name: "orphan_artifacts", path: "/admin/artifacts/operator/orphans" },
    { name: "users", path: "/admin/users/operator?limit=20" },
    { name: "referrals", path: "/admin/referrals/operator?limit=20" },
    { name: "audit", path: "/admin/audit/operator?limit=20" },
    { name: "billing_console", path: "/billing/operator/console?limit=20" },
  ];
  const index = (__VU + __ITER) % routes.length;
  return routes[index];
}

function adminHeaders() {
  return {
    "X-Admin-Token": __ENV.K6_ADMIN_TOKEN || __ENV.ADMIN_TOKEN || "loadtest-admin-token",
  };
}

function isJSONResponse(res) {
  const contentType = String(res.headers["Content-Type"] || res.headers["content-type"] || "");
  return contentType.includes("application/json") || String(res.body || "").trim().startsWith("{") || String(res.body || "").trim().startsWith("[");
}

function containsSensitiveMarkers(res) {
  const body = String(res.body || "").toLowerCase();
  const markers = [
    "authorization",
    "x-admin-token",
    "vk1.",
    "ghp_",
    "yookassa_secret",
    "secret_key",
    "access_token",
    "private_url",
    "raw_payload",
    "prompt_body",
  ];
  return markers.some((marker) => body.includes(marker));
}

function intEnv(name, fallback) {
  const raw = __ENV[name];
  if (raw === undefined || raw === "") {
    return fallback;
  }
  const value = Number.parseInt(raw, 10);
  return Number.isFinite(value) && value > 0 ? value : fallback;
}

function floatEnv(name, fallback) {
  const raw = __ENV[name];
  if (raw === undefined || raw === "") {
    return fallback;
  }
  const value = Number.parseFloat(raw);
  return Number.isFinite(value) && value >= 0 ? value : fallback;
}

function boolEnv(name, fallback) {
  const raw = __ENV[name];
  if (raw === undefined || raw === "") {
    return fallback;
  }
  return ["1", "true", "yes", "on"].includes(String(raw).toLowerCase());
}

function trimTrailingSlash(value) {
  return value.replace(/\/+$/, "");
}

function assertSafeLoadTarget(rawValue, name, allowProduction) {
  const value = String(rawValue || "").trim();
  if (value === "" || allowProduction) {
    return;
  }
  const hostname = hostnameFromURL(value, name);
  if (isProductionHost(hostname)) {
    throw new Error(`${name} points to production host ${hostname}; admin load tests must use local, DEV, staging or loadtest targets`);
  }
}

function hostnameFromURL(value, name) {
  const match = value.match(/^[a-z][a-z0-9+.-]*:\/\/([^/?#:]+)(?::\d+)?(?:[/?#]|$)/i);
  if (!match) {
    throw new Error(`${name} must be an absolute URL for k6 load tests`);
  }
  return match[1].toLowerCase();
}

function isProductionHost(hostname) {
  const host = String(hostname || "").toLowerCase();
  return host === "vk.neiirohub.ru" || host === "app.neiirohub.ru" || host === "neiirohub.ru";
}
