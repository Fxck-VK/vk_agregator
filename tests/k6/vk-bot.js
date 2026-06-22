import http from "k6/http";
import { check, sleep } from "k6";
import { Counter, Rate } from "k6/metrics";

const rawBaseURL = __ENV.K6_BASE_URL || __ENV.BASE_URL || "";
const enabled = rawBaseURL !== "" || __ENV.K6_RUN === "1";
const baseURL = trimTrailingSlash(rawBaseURL || "http://127.0.0.1:8080");
const runID = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

const vkWebhookOK = new Rate("vk_bot_webhook_ok");
const vkDuplicateReplayOK = new Rate("vk_bot_duplicate_replay_ok");
const vkRateBurstEvents = new Counter("vk_bot_rate_burst_events");

export const options = enabled
  ? {
      scenarios: {
        vk_bot_journey: {
          executor: "shared-iterations",
          exec: "vkBotJourney",
          vus: intEnv("K6_VK_BOT_VUS", 1),
          iterations: intEnv("K6_VK_BOT_ITERATIONS", 1),
          maxDuration: __ENV.K6_VK_BOT_MAX_DURATION || "1m",
        },
        vk_rate_limit_burst: {
          executor: "shared-iterations",
          exec: "vkRateLimitBurst",
          vus: intEnv("K6_VK_RATE_LIMIT_VUS", 1),
          iterations: intEnv("K6_VK_RATE_LIMIT_ITERATIONS", 1),
          maxDuration: __ENV.K6_VK_RATE_LIMIT_MAX_DURATION || "1m",
        },
      },
      thresholds: {
        http_req_failed: ["rate<0.05"],
        http_req_duration: ["p(95)<1000"],
        vk_bot_webhook_ok: ["rate>0.99"],
        vk_bot_duplicate_replay_ok: ["rate>0.99"],
      },
      userAgent: "vk-ai-aggregator-k6-vk-bot/1.0",
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
  console.log("k6 VK bot scenario skipped: set K6_BASE_URL to run against a loadtest/dev API.");
}

export function vkBotJourney() {
  const userID = syntheticVKUserID("K6_VK_BOT_USER_BASE", 720000000);
  const peerID = userID;
  const stepSleep = floatEnv("K6_VK_STEP_SLEEP_SECONDS", 0.1);
  const stepPrefix = `k6:${runID}:journey:${__VU}:${__ITER}`;

  const start = postMessage({
    userID,
    peerID,
    eventID: `${stepPrefix}:start`,
    conversationMessageID: conversationID(1),
    text: __ENV.K6_VK_START_TEXT || "/start",
    tag: "start",
  });
  recordWebhookOK(start);
  sleep(stepSleep);

  const duplicateStart = postMessage({
    userID,
    peerID,
    eventID: `${stepPrefix}:start`,
    conversationMessageID: conversationID(1),
    text: __ENV.K6_VK_START_TEXT || "/start",
    tag: "start_duplicate",
  });
  recordDuplicateOK(duplicateStart);
  sleep(stepSleep);

  recordWebhookOK(
    postMessage({
      userID,
      peerID,
      eventID: `${stepPrefix}:show-menu-text`,
      conversationMessageID: conversationID(2),
      text: __ENV.K6_VK_SHOW_MENU_TEXT || "Показать меню",
      tag: "show_menu_text",
    }),
  );
  sleep(stepSleep);

  recordWebhookOK(
    postCallback({
      userID,
      peerID,
      eventID: `${stepPrefix}:show-menu-callback`,
      payload: payloadFromEnv("K6_VK_PAYLOAD_SHOW_MENU", "show_menu"),
      tag: "show_menu_callback",
    }),
  );
  sleep(stepSleep);

  recordWebhookOK(
    postCallback({
      userID,
      peerID,
      eventID: `${stepPrefix}:ask-neurohub`,
      payload: payloadFromEnv("K6_VK_PAYLOAD_ASK_NEUROHUB", "menu.text"),
      tag: "ask_neurohub_callback",
    }),
  );
  sleep(stepSleep);

  const promptEventID = `${stepPrefix}:prompt`;
  recordWebhookOK(
    postMessage({
      userID,
      peerID,
      eventID: promptEventID,
      conversationMessageID: conversationID(3),
      text: __ENV.K6_VK_BOT_PROMPT || "Коротко объясни, что умеет НейроХаб",
      tag: "neurohub_prompt",
    }),
  );
  sleep(stepSleep);

  const duplicatePrompt = postMessage({
    userID,
    peerID,
    eventID: promptEventID,
    conversationMessageID: conversationID(3),
    text: __ENV.K6_VK_BOT_PROMPT || "Коротко объясни, что умеет НейроХаб",
    tag: "neurohub_prompt_duplicate",
  });
  recordDuplicateOK(duplicatePrompt);
  sleep(stepSleep);

  recordWebhookOK(
    postCallback({
      userID,
      peerID,
      eventID: `${stepPrefix}:account`,
      payload: payloadFromEnv("K6_VK_PAYLOAD_ACCOUNT", "account"),
      tag: "account_callback",
    }),
  );
  sleep(stepSleep);

  recordWebhookOK(
    postCallback({
      userID,
      peerID,
      eventID: `${stepPrefix}:video-menu`,
      payload: payloadFromEnv("K6_VK_PAYLOAD_VIDEO_MENU", "menu.video"),
      tag: "video_menu_callback",
    }),
  );
  sleep(stepSleep);

  recordWebhookOK(
    postCallback({
      userID,
      peerID,
      eventID: `${stepPrefix}:back`,
      payload: payloadFromEnv("K6_VK_PAYLOAD_BACK", "show_menu"),
      tag: "back_callback",
    }),
  );
}

export function vkRateLimitBurst() {
  const userID = syntheticVKUserID("K6_VK_RATE_LIMIT_USER_BASE", 730000000);
  const peerID = userID;
  const eventPrefix = `k6:${runID}:rate:${__VU}:${__ITER}`;
  const burstEvents = intEnv("K6_VK_RATE_LIMIT_EVENTS", 45);
  const burstSleep = floatEnv("K6_VK_RATE_LIMIT_SLEEP_SECONDS", 0);

  recordWebhookOK(
    postMessage({
      userID,
      peerID,
      eventID: `${eventPrefix}:start`,
      conversationMessageID: conversationID(1000),
      text: __ENV.K6_VK_START_TEXT || "/start",
      tag: "rate_start",
    }),
  );

  recordWebhookOK(
    postCallback({
      userID,
      peerID,
      eventID: `${eventPrefix}:ask-neurohub`,
      payload: payloadFromEnv("K6_VK_PAYLOAD_ASK_NEUROHUB", "menu.text"),
      tag: "rate_ask_neurohub",
    }),
  );

  for (let i = 0; i < burstEvents; i += 1) {
    const res = postMessage({
      userID,
      peerID,
      eventID: `${eventPrefix}:burst:${i}`,
      conversationMessageID: conversationID(1100 + i),
      text: `${__ENV.K6_VK_RATE_LIMIT_TEXT || "rate limit loadtest"} ${i}`,
      tag: "rate_burst_message",
    });
    recordWebhookOK(res);
    vkRateBurstEvents.add(1);
    sleep(burstSleep);
  }
}

function postMessage({ userID, peerID, eventID, conversationMessageID, text, tag }) {
  const payload = {
    type: "message_new",
    group_id: intEnv("K6_VK_GROUP_ID", 0),
    event_id: eventID,
    secret: vkSecret(),
    object: {
      message: {
        from_id: userID,
        peer_id: peerID,
        conversation_message_id: conversationMessageID,
        text,
      },
    },
  };

  return http.post(`${baseURL}/webhooks/vk`, JSON.stringify(payload), {
    headers: jsonHeaders(),
    tags: { surface: "vk", route: "webhook_message_new", vk_step: tag },
  });
}

function postCallback({ userID, peerID, eventID, payload, tag }) {
  const body = {
    type: "message_event",
    group_id: intEnv("K6_VK_GROUP_ID", 0),
    event_id: eventID,
    secret: vkSecret(),
    object: {
      user_id: userID,
      peer_id: peerID,
      event_id: eventID,
      conversation_message_id: conversationID(9000),
      payload: parsePayload(payload),
    },
  };

  return http.post(`${baseURL}/webhooks/vk`, JSON.stringify(body), {
    headers: jsonHeaders(),
    tags: { surface: "vk", route: "webhook_message_event", vk_step: tag },
  });
}

function payloadFromEnv(name, command) {
  return __ENV[name] || JSON.stringify({ command });
}

function parsePayload(payload) {
  if (typeof payload !== "string") {
    return payload;
  }
  try {
    return JSON.parse(payload);
  } catch (_) {
    return payload;
  }
}

function recordWebhookOK(res) {
  const ok = isVKOK(res);
  vkWebhookOK.add(ok);
  check(res, {
    "VK webhook returns 200": (r) => r.status === 200,
    "VK webhook body is ok": (r) => String(r.body || "").trim() === "ok",
  });
}

function recordDuplicateOK(res) {
  const ok = isVKOK(res);
  vkDuplicateReplayOK.add(ok);
  check(res, {
    "duplicate VK webhook returns 200": (r) => r.status === 200,
    "duplicate VK webhook body is ok": (r) => String(r.body || "").trim() === "ok",
  });
}

function isVKOK(res) {
  return res.status === 200 && String(res.body || "").trim() === "ok";
}

function vkSecret() {
  return __ENV.K6_VK_SECRET || __ENV.VK_SECRET || "loadtest-secret";
}

function jsonHeaders() {
  return { "Content-Type": "application/json" };
}

function syntheticVKUserID(baseEnvName, defaultBase) {
  return intEnv(baseEnvName, defaultBase) + __VU * 100000 + __ITER;
}

function conversationID(offset) {
  return __ITER * 10000 + offset;
}

function intEnv(name, fallback) {
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

function trimTrailingSlash(value) {
  return value.replace(/\/+$/, "");
}
