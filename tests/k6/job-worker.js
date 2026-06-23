import http from "k6/http";
import { check, sleep } from "k6";
import { Counter, Gauge, Rate, Trend } from "k6/metrics";

const rawBaseURL = __ENV.K6_BASE_URL || __ENV.BASE_URL || "";
const allowProductionLiveSmoke = boolEnv("K6_ALLOW_PRODUCTION_LIVE_SMOKE", false);
assertSafeLoadTarget(rawBaseURL, "K6_BASE_URL/BASE_URL", allowProductionLiveSmoke);
const enabled = rawBaseURL !== "" || __ENV.K6_RUN === "1";
const baseURL = trimTrailingSlash(rawBaseURL || "http://127.0.0.1:8080");
const runID = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

const jobCreated = new Counter("job_created_total");
const jobTerminal = new Counter("job_terminal_total");
const jobTerminalFailure = new Counter("job_terminal_failure_total");
const jobCreateOK = new Rate("job_create_ok");
const jobPollOK = new Rate("job_poll_ok");
const jobSuccessOK = new Rate("job_success_ok");
const jobCreateDuration = new Trend("job_create_duration");
const jobCompletionDuration = new Trend("job_completion_duration");
const jobPendingAfterPoll = new Gauge("job_pending_after_poll");

export const options = enabled
  ? {
      scenarios: buildScenarios(),
      thresholds: {
        http_req_failed: ["rate<0.05"],
        http_req_duration: ["p(95)<1500"],
        job_create_ok: ["rate>0.95"],
        job_poll_ok: ["rate>0.90"],
        job_success_ok: ["rate>0.95"],
      },
      userAgent: "vk-ai-aggregator-k6-job-worker/1.0",
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
  console.log("k6 job/worker scenario skipped: set K6_BASE_URL to run against a loadtest/dev API.");
}

export function textJobs() {
  createAndPollJob("text_generate");
}

export function imageJobs() {
  createAndPollJob("image_generate");
}

export function videoJobs() {
  createAndPollJob("video_generate");
}

export function mixedJobs() {
  const textWeight = nonNegativeIntEnv("K6_JOB_TEXT_WEIGHT", 60);
  const imageWeight = nonNegativeIntEnv("K6_JOB_IMAGE_WEIGHT", 25);
  const videoWeight = nonNegativeIntEnv("K6_JOB_VIDEO_WEIGHT", 15);
  const total = Math.max(1, textWeight + imageWeight + videoWeight);
  const roll = Math.random() * total;

  if (roll < textWeight) {
    createAndPollJob("text_generate");
    return;
  }
  if (roll < textWeight + imageWeight) {
    createAndPollJob("image_generate");
    return;
  }
  createAndPollJob("video_generate");
}

function createAndPollJob(operation) {
  const vkUserID = syntheticVKUserID();
  const headers = {
    ...miniappHeaders(vkUserID),
    "Content-Type": "application/json",
    "X-Idempotency-Key": `k6.job.${runID}.${operation}.${__VU}.${__ITER}.${randomSuffix()}`,
  };
  const payload = jobPayload(operation);

  const started = Date.now();
  const created = http.post(`${baseURL}/miniapp/jobs`, JSON.stringify(payload), {
    headers,
    tags: { surface: "miniapp", route: "jobs_create", operation },
  });
  jobCreateDuration.add(created.timings.duration, { operation });

  const jobID = jsonField(created, "id");
  const createdOK = created.status === 201 && typeof jobID === "string" && jobID.length > 0;
  jobCreateOK.add(createdOK, { operation });
  check(created, {
    "POST /miniapp/jobs is 201": (r) => r.status === 201,
    "POST /miniapp/jobs returns id": () => typeof jobID === "string" && jobID.length > 0,
  });

  if (!createdOK) {
    sleep(floatEnv("K6_JOB_SLEEP_SECONDS", 0));
    return;
  }

  jobCreated.add(1, { operation });
  if (boolEnv("K6_JOB_POLL", true)) {
    pollJob(jobID, headers, operation, started);
  }

  sleep(floatEnv("K6_JOB_SLEEP_SECONDS", 0));
}

function pollJob(jobID, headers, operation, started) {
  const attempts = intEnv("K6_JOB_POLL_ATTEMPTS", 10);
  const interval = floatEnv("K6_JOB_POLL_INTERVAL_SECONDS", 0.5);
  let lastStatus = "";

  for (let i = 0; i < attempts; i += 1) {
    const res = http.get(`${baseURL}/miniapp/jobs/${encodeURIComponent(jobID)}`, {
      headers,
      tags: { surface: "miniapp", route: "jobs_get", operation },
    });
    const pollOK = res.status === 200;
    jobPollOK.add(pollOK, { operation });
    check(res, {
      "GET /miniapp/jobs/{id} is 200": (r) => r.status === 200,
      "GET /miniapp/jobs/{id} returns same id": (r) => jsonField(r, "id") === jobID,
    });

    lastStatus = String(jsonField(res, "status") || "");
    if (isTerminalStatus(lastStatus)) {
      const succeeded = isSuccessfulTerminalStatus(lastStatus);
      jobTerminal.add(1, { operation, status: lastStatus });
      jobSuccessOK.add(succeeded, { operation, status: lastStatus });
      if (!succeeded) {
        jobTerminalFailure.add(1, { operation, status: lastStatus });
      }
      jobCompletionDuration.add(Date.now() - started, { operation, status: lastStatus });
      jobPendingAfterPoll.add(0, { operation });
      return;
    }

    sleep(interval);
  }

  jobPendingAfterPoll.add(1, { operation, status: lastStatus || "unknown" });
}

function jobPayload(operation) {
  const payload = {
    operation,
    prompt: promptFor(operation),
  };

  if (operation === "image_generate") {
    const modelID = __ENV.K6_JOB_IMAGE_MODEL_ID || "";
    if (modelID !== "") {
      payload.model_id = modelID;
    }
  }

  if (operation === "video_generate") {
    // Mini App video API accepts public route aliases, not provider model IDs.
    payload.video_route_alias = __ENV.K6_JOB_VIDEO_ROUTE_ALIAS || "video_mock_text_to_video";
    payload.duration_sec = intEnv("K6_JOB_VIDEO_DURATION_SEC", 5);
  }

  return {
    ...payload,
    ...extraPayload(operation),
  };
}

function promptFor(operation) {
  switch (operation) {
    case "image_generate":
      return __ENV.K6_JOB_IMAGE_PROMPT || "load-test image prompt: neon cat in glasses";
    case "video_generate":
      return __ENV.K6_JOB_VIDEO_PROMPT || "load-test video prompt: robot walking through a neon city";
    default:
      return __ENV.K6_JOB_TEXT_PROMPT || "load-test text prompt: answer briefly";
  }
}

function extraPayload(operation) {
  const envName = {
    text_generate: "K6_JOB_TEXT_EXTRA_JSON",
    image_generate: "K6_JOB_IMAGE_EXTRA_JSON",
    video_generate: "K6_JOB_VIDEO_EXTRA_JSON",
  }[operation];
  const raw = envName ? __ENV[envName] : "";
  if (!raw || raw === "{}") {
    return {};
  }
  try {
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : {};
  } catch (err) {
    console.warn(`invalid ${envName}: ${err}`);
    return {};
  }
}

function buildScenarios() {
  const workload = (__ENV.K6_JOB_WORKLOAD || "mixed").toLowerCase();
  const duration = __ENV.K6_JOB_DURATION || "30s";

  if (workload === "text") {
    return { text_jobs: arrivalScenario("textJobs", "K6_JOB_TEXT_RATE", duration) };
  }
  if (workload === "image") {
    return { image_jobs: arrivalScenario("imageJobs", "K6_JOB_IMAGE_RATE", duration) };
  }
  if (workload === "video") {
    return { video_jobs: arrivalScenario("videoJobs", "K6_JOB_VIDEO_RATE", duration) };
  }
  if (workload === "all") {
    return {
      text_jobs: arrivalScenario("textJobs", "K6_JOB_TEXT_RATE", duration),
      image_jobs: arrivalScenario("imageJobs", "K6_JOB_IMAGE_RATE", duration),
      video_jobs: arrivalScenario("videoJobs", "K6_JOB_VIDEO_RATE", duration),
    };
  }

  return {
    mixed_jobs: arrivalScenario("mixedJobs", "K6_JOB_MIXED_RATE", duration),
  };
}

function arrivalScenario(exec, rateEnvName, duration) {
  const rate = intEnv(rateEnvName, 1);
  const preAllocatedVUs = intEnv(`${rateEnvName}_PREALLOCATED_VUS`, Math.max(2, rate * 2));
  const maxVUs = intEnv(`${rateEnvName}_MAX_VUS`, Math.max(preAllocatedVUs, rate * 6, 10));

  return {
    executor: "constant-arrival-rate",
    exec,
    rate,
    timeUnit: "1s",
    duration,
    preAllocatedVUs,
    maxVUs,
  };
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

function syntheticVKUserID() {
  const base = intEnv("K6_JOB_USER_BASE", 820000000);
  if (boolEnv("K6_JOB_SAME_USER", false)) {
    return base;
  }
  return base + (__VU * 100000) + (__ITER % 100000);
}

function isTerminalStatus(status) {
  return [
    "succeeded",
    "completed",
    "delivered",
    "failed",
    "failed_terminal",
    "canceled",
    "cancelled",
  ].includes(status);
}

function isSuccessfulTerminalStatus(status) {
  return ["succeeded", "completed", "delivered"].includes(status);
}

function jsonField(res, field) {
  try {
    return res.json(field);
  } catch (_) {
    return undefined;
  }
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

function randomSuffix() {
  return Math.random().toString(36).slice(2, 10);
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
  return host === "vk.neiirohub.ru" || host === "app.neiirohub.ru" || host === "neiirohub.ru";
}
