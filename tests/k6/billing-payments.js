import http from "k6/http";
import { check, sleep } from "k6";
import { Counter, Rate, Trend } from "k6/metrics";

const rawBaseURL = __ENV.K6_BASE_URL || __ENV.BASE_URL || "";
const enabled = rawBaseURL !== "" || __ENV.K6_RUN === "1";
const baseURL = trimTrailingSlash(rawBaseURL || "http://127.0.0.1:8080");
const webhookBaseURL = trimTrailingSlash(__ENV.K6_PAYMENT_WEBHOOK_BASE_URL || "");
const runID = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

const intentCreated = new Counter("payment_intent_created_total");
const historyOK = new Rate("payment_history_ok");
const mockTopupOK = new Rate("payment_mock_topup_ok");
const refundOK = new Rate("payment_refund_ok");
const idempotencyOK = new Rate("payment_idempotency_ok");
const journeyDuration = new Trend("payment_mock_journey_duration");

export const options = enabled
  ? {
      scenarios: {
        billing_payments_mock: {
          executor: "constant-arrival-rate",
          exec: "billingPaymentsMock",
          rate: intEnv("K6_PAYMENT_RATE", 1),
          timeUnit: "1s",
          duration: __ENV.K6_PAYMENT_DURATION || __ENV.K6_DURATION || "30s",
          preAllocatedVUs: intEnv("K6_PAYMENT_PREALLOCATED_VUS", 5),
          maxVUs: intEnv("K6_PAYMENT_MAX_VUS", 20),
        },
      },
      thresholds: {
        http_req_failed: ["rate<0.05"],
        http_req_duration: ["p(95)<1500"],
        payment_history_ok: ["rate>0.95"],
        payment_mock_topup_ok: ["rate>0.95"],
        payment_refund_ok: ["rate>0.95"],
        payment_idempotency_ok: ["rate>0.95"],
      },
      userAgent: "vk-ai-aggregator-k6-billing-payments/1.0",
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
  console.log("k6 billing/payment scenario skipped: set K6_BASE_URL to run against a loadtest/dev API.");
}

export function billingPaymentsMock() {
  const started = Date.now();
  const vkUserID = syntheticVKUserID();
  const userHeaders = miniappHeaders(vkUserID);
  const beforeBalance = getBalanceCredits(userHeaders);
  const productCode = __ENV.K6_PAYMENT_PRODUCT_CODE || "crystals_99";
  const idempotencyKey = `k6.payment.${runID}.${__VU}.${__ITER}`;

  const firstIntent = createPaymentIntent(userHeaders, productCode, idempotencyKey);
  if (!firstIntent.ok) {
    sleep(floatEnv("K6_PAYMENT_SLEEP_SECONDS", 0));
    return;
  }
  intentCreated.add(1, { product_code: productCode });

  const replayIntent = createPaymentIntent(userHeaders, productCode, idempotencyKey);
  const sameIntent = replayIntent.ok && replayIntent.id === firstIntent.id;
  idempotencyOK.add(sameIntent, { step: "create_intent" });
  check(replayIntent.response, {
    "payment intent replay returns same id": () => sameIntent,
  });

  const history = http.get(`${baseURL}/miniapp/payments?limit=10`, {
    headers: userHeaders,
    tags: { surface: "miniapp", route: "payment_history" },
  });
  const historyGood = history.status === 200 && Array.isArray(jsonField(history, "items"));
  historyOK.add(historyGood);
  check(history, {
    "GET /miniapp/payments is 200": (r) => r.status === 200,
    "GET /miniapp/payments returns items": () => historyGood,
  });

  const adminIntent = getAdminIntent(firstIntent.id);
  if (!adminIntent.ok) {
    mockTopupOK.add(false);
    refundOK.add(false);
    sleep(floatEnv("K6_PAYMENT_SLEEP_SECONDS", 0));
    return;
  }

  const topup = setMockStatus(firstIntent.id, "succeeded", "loadtest mock payment completion");
  const topupReplay = setMockStatus(firstIntent.id, "succeeded", "loadtest mock payment completion");
  const topupGood = topup.ok && topupReplay.ok && topup.status === "succeeded" && topupReplay.status === "succeeded";
  mockTopupOK.add(topupGood);
  idempotencyOK.add(topupGood, { step: "mock_status_replay" });
  check(topup.response, {
    "mock status topup is 200": (r) => r.status === 200,
    "mock status topup returns succeeded": () => topup.status === "succeeded",
  });

  if (webhookBaseURL !== "" && adminIntent.providerPaymentID) {
    replayPaymentWebhook(adminIntent.providerPaymentID);
  }

  const afterTopupBalance = getBalanceCredits(userHeaders);
  if (beforeBalance !== undefined && afterTopupBalance !== undefined && firstIntent.credits !== undefined) {
    check(null, {
      "balance increased by credits after mock topup": () => afterTopupBalance >= beforeBalance + firstIntent.credits,
    });
  }

  const refund = refundIntent(firstIntent.id);
  const refundReplay = refundIntent(firstIntent.id);
  const refundGood = refund.ok && refundReplay.ok && refund.refundID === refundReplay.refundID;
  refundOK.add(refundGood);
  idempotencyOK.add(refundGood, { step: "refund_replay" });
  check(refund.response, {
    "refund mock is 200": (r) => r.status === 200,
    "refund replay returns same refund id": () => refundGood,
  });

  if (webhookBaseURL !== "" && refund.providerRefundID) {
    replayRefundWebhook(refund.providerRefundID);
  }

  journeyDuration.add(Date.now() - started);
  sleep(floatEnv("K6_PAYMENT_SLEEP_SECONDS", 0));
}

function createPaymentIntent(headers, productCode, idempotencyKey) {
  const payload = {
    product_code: productCode,
    receipt_email: __ENV.K6_PAYMENT_RECEIPT_EMAIL || "loadtest@example.com",
    return_url: __ENV.K6_PAYMENT_RETURN_URL || "https://loadtest.local/payments/return",
    force_new: boolEnv("K6_PAYMENT_FORCE_NEW", true),
  };
  const res = http.post(`${baseURL}/miniapp/payments/intents`, JSON.stringify(payload), {
    headers: {
      ...headers,
      "Content-Type": "application/json",
      "X-Idempotency-Key": idempotencyKey,
    },
    tags: { surface: "miniapp", route: "payment_intent_create", product_code: productCode },
  });
  const id = jsonField(res, "id");
  const ok = (res.status === 200 || res.status === 201) && typeof id === "string" && id.length > 0;
  check(res, {
    "POST /miniapp/payments/intents is 200/201": (r) => r.status === 200 || r.status === 201,
    "POST /miniapp/payments/intents returns id": () => typeof id === "string" && id.length > 0,
  });
  return {
    ok,
    response: res,
    id,
    status: jsonField(res, "status"),
    credits: numberField(res, "credits"),
  };
}

function getAdminIntent(intentID) {
  const res = http.get(`${baseURL}/billing/payment-intents/${encodeURIComponent(intentID)}`, {
    headers: adminHeaders(),
    tags: { surface: "billing", route: "payment_intent_get" },
  });
  const ok = res.status === 200;
  check(res, {
    "GET /billing/payment-intents/{id} is 200": (r) => r.status === 200,
  });
  return {
    ok,
    response: res,
    providerPaymentID: jsonField(res, "provider_payment_id"),
  };
}

function setMockStatus(intentID, status, reason) {
  const res = http.post(
    `${baseURL}/billing/payment-intents/${encodeURIComponent(intentID)}/mock-status`,
    JSON.stringify({ status, reason }),
    {
      headers: {
        ...adminHeaders(),
        "Content-Type": "application/json",
        "X-Idempotency-Key": `k6.payment.${runID}.mock-status.${intentID}`,
      },
      tags: { surface: "billing", route: "payment_intent_mock_status", status },
    },
  );
  return {
    ok: res.status === 200,
    response: res,
    status: jsonField(res, "status"),
  };
}

function refundIntent(intentID) {
  const res = http.post(
    `${baseURL}/billing/payment-intents/${encodeURIComponent(intentID)}/refund`,
    JSON.stringify({ reason: "loadtest refund replay" }),
    {
      headers: {
        ...adminHeaders(),
        "Content-Type": "application/json",
        "X-Idempotency-Key": `k6.payment.${runID}.refund.${intentID}`,
      },
      tags: { surface: "billing", route: "payment_intent_refund" },
    },
  );
  const body = safeJSON(res);
  const refund = body && body.refund ? body.refund : {};
  return {
    ok: res.status === 200,
    response: res,
    refundID: refund.id,
    providerRefundID: refund.provider_refund_id,
  };
}

function replayPaymentWebhook(providerPaymentID) {
  const payload = {
    event_type: "payment.succeeded",
    provider_payment_id: providerPaymentID,
  };
  postWebhookReplay(payload, "payment_succeeded_replay");
  postWebhookReplay(payload, "payment_succeeded_replay");
}

function replayRefundWebhook(providerRefundID) {
  const payload = {
    event_type: "refund.succeeded",
    provider_refund_id: providerRefundID,
  };
  postWebhookReplay(payload, "refund_succeeded_replay");
  postWebhookReplay(payload, "refund_succeeded_replay");
}

function postWebhookReplay(payload, route) {
  const res = http.post(`${webhookBaseURL}/billing/webhooks/yookassa`, JSON.stringify(payload), {
    headers: { "Content-Type": "application/json" },
    tags: { surface: "payment_webhook", route },
  });
  check(res, {
    [`POST /billing/webhooks/yookassa ${route} accepts replay`]: (r) => r.status >= 200 && r.status < 300,
  });
}

function getBalanceCredits(headers) {
  const res = http.get(`${baseURL}/miniapp/balance`, {
    headers,
    tags: { surface: "miniapp", route: "balance" },
  });
  check(res, {
    "GET /miniapp/balance is 200": (r) => r.status === 200,
  });
  return numberField(res, "balance_credits");
}

function adminHeaders() {
  return {
    "X-Admin-Token": __ENV.K6_ADMIN_TOKEN || __ENV.ADMIN_TOKEN || "loadtest-admin-token",
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

function jsonField(res, field) {
  try {
    return res.json(field);
  } catch (_) {
    return undefined;
  }
}

function safeJSON(res) {
  try {
    return res.json();
  } catch (_) {
    return undefined;
  }
}

function numberField(res, field) {
  const value = jsonField(res, field);
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function syntheticVKUserID() {
  if (boolEnv("K6_PAYMENT_SAME_USER", false)) {
    return intEnv("K6_PAYMENT_USER_BASE", 840000000);
  }
  return intEnv("K6_PAYMENT_USER_BASE", 840000000) + (__VU * 100000) + __ITER;
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
