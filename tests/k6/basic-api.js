import http from "k6/http";
import { check, sleep } from "k6";

const rawBaseURL = __ENV.K6_BASE_URL || __ENV.BASE_URL || "";
const enabled = rawBaseURL !== "" || __ENV.K6_RUN === "1";
const baseURL = trimTrailingSlash(rawBaseURL || "http://127.0.0.1:8080");
const runID = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

export const options = enabled
  ? {
      scenarios: {
        health_readiness: {
          executor: "constant-vus",
          exec: "healthReadiness",
          vus: intEnv("K6_HEALTH_VUS", 1),
          duration: __ENV.K6_DURATION || "30s",
        },
        vk_webhook: {
          executor: "constant-vus",
          exec: "vkWebhook",
          vus: intEnv("K6_VK_VUS", 1),
          duration: __ENV.K6_DURATION || "30s",
        },
        miniapp_jobs: {
          executor: "constant-vus",
          exec: "miniappJobs",
          vus: intEnv("K6_MINIAPP_VUS", 1),
          duration: __ENV.K6_DURATION || "30s",
        },
      },
      thresholds: {
        http_req_failed: ["rate<0.05"],
        http_req_duration: ["p(95)<1000"],
      },
      userAgent: "vk-ai-aggregator-k6-basic/1.0",
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
  console.log("k6 basic API scenario skipped: set K6_BASE_URL to run against a loadtest/dev API.");
}

export function healthReadiness() {
  const health = http.get(`${baseURL}/health`, { tags: { surface: "api", route: "health" } });
  check(health, {
    "GET /health is 200": (r) => r.status === 200,
  });

  const readyz = http.get(`${baseURL}/readyz`, { tags: { surface: "api", route: "readyz" } });
  check(readyz, {
    "GET /readyz is 200": (r) => r.status === 200,
  });

  sleep(floatEnv("K6_SLEEP_SECONDS", 1));
}

export function vkWebhook() {
  const userID = syntheticVKUserID("K6_VK_USER_BASE", 710000000);
  const payload = {
    type: "message_new",
    group_id: intEnv("K6_VK_GROUP_ID", 0),
    event_id: `k6:${runID}:vk:${__VU}:${__ITER}`,
    secret: __ENV.K6_VK_SECRET || __ENV.VK_SECRET || "loadtest-secret",
    object: {
      message: {
        from_id: userID,
        peer_id: userID,
        conversation_message_id: __ITER + 1,
        text: __ENV.K6_VK_TEXT || "show menu",
      },
    },
  };

  const res = http.post(`${baseURL}/webhooks/vk`, JSON.stringify(payload), {
    headers: jsonHeaders(),
    tags: { surface: "vk", route: "webhook_message_new" },
  });

  check(res, {
    "POST /webhooks/vk is 200": (r) => r.status === 200,
    "POST /webhooks/vk returns ok": (r) => String(r.body || "").trim() === "ok",
  });

  sleep(floatEnv("K6_SLEEP_SECONDS", 1));
}

export function miniappJobs() {
  const vkUserID = syntheticVKUserID("K6_MINIAPP_USER_BASE", 810000000);
  const headers = miniappHeaders(vkUserID);

  const balance = http.get(`${baseURL}/miniapp/balance`, {
    headers,
    tags: { surface: "miniapp", route: "balance" },
  });
  check(balance, {
    "GET /miniapp/balance is 200": (r) => r.status === 200,
    "GET /miniapp/balance has balance_credits": (r) => jsonFieldExists(r, "balance_credits"),
  });

  const createHeaders = {
    ...headers,
    "Content-Type": "application/json",
    "X-Idempotency-Key": `k6.${runID}.miniapp.${__VU}.${__ITER}`,
  };
  const createPayload = {
    operation: "text_generate",
    prompt: __ENV.K6_MINIAPP_PROMPT || "basic load-test prompt",
  };

  const created = http.post(`${baseURL}/miniapp/jobs`, JSON.stringify(createPayload), {
    headers: createHeaders,
    tags: { surface: "miniapp", route: "jobs_create" },
  });
  const jobID = jsonField(created, "id");

  check(created, {
    "POST /miniapp/jobs is 201": (r) => r.status === 201,
    "POST /miniapp/jobs returns id": () => typeof jobID === "string" && jobID.length > 0,
  });

  if (typeof jobID === "string" && jobID.length > 0) {
    const job = http.get(`${baseURL}/miniapp/jobs/${encodeURIComponent(jobID)}`, {
      headers,
      tags: { surface: "miniapp", route: "jobs_get" },
    });
    check(job, {
      "GET /miniapp/jobs/{id} is 200": (r) => r.status === 200,
      "GET /miniapp/jobs/{id} returns same id": (r) => jsonField(r, "id") === jobID,
    });
  }

  sleep(floatEnv("K6_SLEEP_SECONDS", 1));
}

function miniappHeaders(vkUserID) {
  const headers = {};
  const launchParams = __ENV.K6_MINIAPP_LAUNCH_PARAMS || __ENV.MINIAPP_LAUNCH_PARAMS || "";
  if (launchParams !== "") {
    headers["X-Launch-Params"] = launchParams;
    return headers;
  }
  headers["X-VK-User-ID"] = String(vkUserID);
  return headers;
}

function jsonHeaders() {
  return { "Content-Type": "application/json" };
}

function jsonField(res, field) {
  try {
    return res.json(field);
  } catch (_) {
    return undefined;
  }
}

function jsonFieldExists(res, field) {
  return jsonField(res, field) !== undefined;
}

function syntheticVKUserID(baseEnvName, defaultBase) {
  return intEnv(baseEnvName, defaultBase) + (__VU * 100000) + __ITER;
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

function trimTrailingSlash(value) {
  return value.replace(/\/+$/, "");
}
