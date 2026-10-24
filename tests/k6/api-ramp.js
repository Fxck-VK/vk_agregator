import http from "k6/http";
import { check } from "k6";
import { Rate } from "k6/metrics";

const rawBaseURL = __ENV.K6_BASE_URL || __ENV.BASE_URL || "";
const allowProductionLiveSmoke = boolEnv("K6_ALLOW_PRODUCTION_LIVE_SMOKE", false);
assertSafeLoadTarget(rawBaseURL, "K6_BASE_URL/BASE_URL", allowProductionLiveSmoke);
const enabled = rawBaseURL !== "" || __ENV.K6_RUN === "1";
const baseURL = trimTrailingSlash(rawBaseURL || "http://127.0.0.1:8080");
const runID = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

const healthOK = new Rate("ramp_health_ok");
const vkOK = new Rate("ramp_vk_ok");

export const options = enabled
  ? {
      scenarios: {
        api_ramp: {
          executor: "constant-arrival-rate",
          exec: "apiRamp",
          rate: intEnv("K6_RAMP_RPS", 10),
          timeUnit: "1s",
          duration: __ENV.K6_RAMP_DURATION || __ENV.K6_CAPACITY_SUSTAIN_DURATION || "2m",
          preAllocatedVUs: intEnv("K6_RAMP_PREALLOCATED_VUS", Math.max(10, intEnv("K6_RAMP_RPS", 10) * 2)),
          maxVUs: intEnv("K6_RAMP_MAX_VUS", Math.max(20, intEnv("K6_RAMP_RPS", 10) * 6)),
        },
      },
      thresholds: {
        http_req_failed: ["rate<0.05"],
        http_req_duration: ["p(95)<1500"],
        ramp_health_ok: ["rate>0.95"],
        ramp_vk_ok: ["rate>0.95"],
      },
      userAgent: "vk-ai-aggregator-k6-api-ramp/1.0",
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
  console.log("k6 API ramp skipped: set K6_BASE_URL to run against a loadtest/dev API.");
}


export function apiRamp() {
  if (Math.random() < 0.4) healthReadiness();
  else vkWebhook();
}

function healthReadiness() {
  const health = http.get(`${baseURL}/health`, { tags: { surface: "api", route: "health" } });
  const healthPass = check(health, {
    "ramp GET /health is 200": (r) => r.status === 200,
  });

  const readyz = http.get(`${baseURL}/readyz`, { tags: { surface: "api", route: "readyz" } });
  const readyPass = check(readyz, {
    "ramp GET /readyz is 200": (r) => r.status === 200,
  });

  healthOK.add(healthPass && readyPass);
}

function vkWebhook() {
  const userID = syntheticVKUserID("K6_RAMP_VK_USER_BASE", 910000000);
  const payload = {
    type: "message_new",
    group_id: intEnv("K6_VK_GROUP_ID", 0),
    event_id: `k6:${runID}:ramp:vk:${__VU}:${__ITER}`,
    secret: __ENV.K6_VK_SECRET || __ENV.VK_SECRET || "loadtest-secret",
    object: {
      message: {
        from_id: userID,
        peer_id: userID,
        conversation_message_id: __ITER + 1,
        text: __ENV.K6_RAMP_VK_TEXT || "loadtest menu",
      },
    },
  };

  const res = http.post(`${baseURL}/webhooks/vk`, JSON.stringify(payload), {
    headers: jsonHeaders(),
    tags: { surface: "vk", route: "webhook_message_new" },
  });
  const ok = check(res, {
    "ramp POST /webhooks/vk is 200": (r) => r.status === 200,
    "ramp POST /webhooks/vk returns ok": (r) => String(r.body || "").trim() === "ok",
  });
  vkOK.add(ok);
}





function jsonHeaders() {
  return { "Content-Type": "application/json" };
}



function syntheticVKUserID(baseEnvName, defaultBase) {
  return intEnv(baseEnvName, defaultBase) + (__VU * 100000) + (__ITER % 100000);
}

function intEnv(name, fallback) {
  const raw = __ENV[name];
  if (raw === undefined || raw === "") {
    return fallback;
  }
  const value = Number.parseInt(raw, 10);
  return Number.isFinite(value) && value > 0 ? value : fallback;
}

function nonNegativeIntEnv(name, fallback) {
  const raw = __ENV[name];
  if (raw === undefined || raw === "") {
    return fallback;
  }
  const value = Number.parseInt(raw, 10);
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
    throw new Error(`${name} points to production host ${hostname}; generic load tests must use local, DEV, staging or loadtest targets`);
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
  return host === "vk.epifera.com" || host === "app.epifera.com" || host === "epifera.com";
}
