const playwright = await import("@playwright/test").catch(() => import("../../miniapp/node_modules/@playwright/test/index.mjs"));
const { expect, test } = playwright;

const NOW = "2026-06-13T08:00:00Z";
const TEST_OPERATOR_AUTH = "stage9_operator_auth_must_not_render";
const RAW_PROMPT = "stage9_raw_prompt_must_not_render";
const RAW_PROVIDER_PAYLOAD = "stage9_raw_provider_payload_must_not_render";
const PRIVATE_ARTIFACT_URL = "stage9_private_artifact_url_must_not_render";
const RAW_PAYMENT_PAYLOAD = "stage9_raw_payment_payload_must_not_render";
const RAW_VK_USER_ID = "777777000";
const RAW_STACK = "stage9_raw_stack_trace_must_not_render";
const IDEMPOTENCY_MARKER = "stage9_idempotency_marker_must_not_render";
const ACTION_REF = "opact_v1_stage9_refund_action";

const forbiddenMarkers = [
  TEST_OPERATOR_AUTH,
  RAW_PROMPT,
  RAW_PROVIDER_PAYLOAD,
  PRIVATE_ARTIFACT_URL,
  RAW_PAYMENT_PAYLOAD,
  RAW_VK_USER_ID,
  RAW_STACK,
  IDEMPOTENCY_MARKER,
];

type SmokeCapture = {
  consoleMessages: string[];
  networkOutput: string[];
};

async function fulfillJSON(route: any, body: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

async function setupMockBackend(page: any): Promise<SmokeCapture> {
  const capture: SmokeCapture = { consoleMessages: [], networkOutput: [] };

  page.on("console", (message) => {
    capture.consoleMessages.push(`${message.type()}:${message.text()}`);
  });
  page.on("pageerror", (error) => {
    capture.consoleMessages.push(`pageerror:${error.message}`);
  });
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (url.pathname.startsWith("/admin") || url.pathname.startsWith("/billing")) {
      capture.networkOutput.push(`${request.method()} ${url.pathname}${url.search} ${request.postData() ?? ""}`);
    }
  });

  await page.route("**/admin/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method();

    if (method !== "GET") {
      await fulfillJSON(route, { error: "method_not_allowed", raw_stack: RAW_STACK }, 405);
      return;
    }
    if (path === "/admin/overview") {
      await fulfillJSON(route, overviewDTO());
      return;
    }
    if (path === "/admin/jobs/operator") {
      await fulfillJSON(route, jobsDTO());
      return;
    }
    if (path === "/admin/jobs/lookup_stage9_job/operator") {
      await fulfillJSON(route, jobDetailDTO());
      return;
    }
    if (path === "/admin/providers/operator") {
      await fulfillJSON(route, providersDTO());
      return;
    }
    if (path === "/admin/media-safety/operator") {
      await fulfillJSON(route, mediaSafetyDTO());
      return;
    }
    await fulfillJSON(route, { error: "not_found", raw_payload: RAW_PROVIDER_PAYLOAD }, 404);
  });

  await page.route("**/billing/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method();

    if (path === "/billing/operator/console" && method === "GET") {
      await fulfillJSON(route, paymentsDTO());
      return;
    }
    if (path === `/billing/payment-intents/${ACTION_REF}/refund` && method === "POST") {
      await fulfillJSON(route, {
        intent: { status: "refunded", raw_provider_payload: RAW_PAYMENT_PAYLOAD },
        refund: { status: "succeeded", idempotency_key: IDEMPOTENCY_MARKER },
      });
      return;
    }
    await fulfillJSON(route, { error: "not_found", raw_payment_payload: RAW_PAYMENT_PAYLOAD }, 404);
  });

  await page.route("**/health", (route) => route.fulfill({ status: 200, body: "ok" }));
  await page.route("**/healthz", (route) => route.fulfill({ status: 200, body: "ok" }));

  return capture;
}

test("admin operator smoke keeps sensitive data out of UI, console, network bodies and storage", async ({ page }) => {
  const capture = await setupMockBackend(page);

  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Overview is locked" })).toBeVisible();
  expect(await localStorageSnapshot(page)).not.toContain(TEST_OPERATOR_AUTH);

  await page.getByLabel("Admin token").fill(TEST_OPERATOR_AUTH);
  await page.getByRole("button", { name: "Use token" }).click();
  await expect(page.getByRole("heading", { name: "API", exact: true })).toBeVisible();
  await expect(page.getByText("in memory")).toBeVisible();
  expect(await localStorageSnapshot(page)).not.toContain(TEST_OPERATOR_AUTH);

  await page.getByRole("button", { name: "Jobs Execution state" }).click();
  await expect(page.getByRole("heading", { name: "job_stage9_display" })).toBeVisible();
  await expect(page.getByText("delivery_timeout").first()).toBeVisible();

  await page.getByRole("button", { name: "Payments Ledger-backed billing" }).click();
  await expect(page.getByRole("heading", { name: "Payments" })).toBeVisible();
  await page.getByRole("button", { name: "Refund" }).last().click();
  await expect(page.getByRole("dialog", { name: "Refund confirmation" })).toBeVisible();
  await page.getByPlaceholder("Short operational reason without secrets, URLs, payloads or user PII").fill(
    "operator verifies safe full refund policy",
  );
  await page.getByRole("button", { name: "Confirm" }).click();
  await expect(page.getByText("Refund completed. The snapshot was refreshed from backend state.")).toBeVisible();

  await page.getByRole("button", { name: "Providers Model health" }).click();
  await expect(page.getByRole("heading", { name: "Control room" })).toBeVisible();
  await expect(page.getByText("deepinfra", { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "Media Safety Upload and video policy" }).click();
  await expect(page.getByRole("heading", { name: "Worker-owned safety config" })).toBeVisible();
  await expect(page.getByText("Probe / transcode / fast path")).toBeVisible();

  await assertNoSensitiveOutput(page, capture);
});

test("admin operator smoke renders backend failures as safe errors", async ({ page }) => {
  const capture: SmokeCapture = { consoleMessages: [], networkOutput: [] };
  page.on("console", (message) => capture.consoleMessages.push(`${message.type()}:${message.text()}`));
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (url.pathname.startsWith("/admin")) {
      capture.networkOutput.push(`${request.method()} ${url.pathname} ${request.postData() ?? ""}`);
    }
  });
  await page.route("**/admin/overview", (route) =>
    fulfillJSON(
      route,
      {
        error: "provider failed",
        token: TEST_OPERATOR_AUTH,
        prompt: RAW_PROMPT,
        private_url: PRIVATE_ARTIFACT_URL,
        stack: RAW_STACK,
      },
      502,
    ),
  );

  await page.goto("/");
  await page.getByLabel("Admin token").fill(TEST_OPERATOR_AUTH);
  await page.getByRole("button", { name: "Use token" }).click();
  await expect(page.getByRole("heading", { name: "Admin service is unavailable." })).toBeVisible();

  await assertNoSensitiveOutput(page, capture);
});

async function assertNoSensitiveOutput(page: any, capture: SmokeCapture) {
  const dom = await page.locator("body").innerText();
  const html = await page.locator("#root").evaluate((node) => node.innerHTML);
  const storage = await localStorageSnapshot(page);
  const output = [dom, html, storage, ...capture.consoleMessages, ...capture.networkOutput].join("\n");

  for (const marker of forbiddenMarkers) {
    expect(output).not.toContain(marker);
  }
  expect(html).not.toContain("<script");
  expect(html).not.toContain("onerror=");
  expect(html).not.toContain("onclick=");
}

async function localStorageSnapshot(page: any): Promise<string> {
  return page.evaluate(() => {
    const values: Record<string, string | null> = {};
    for (let index = 0; index < localStorage.length; index += 1) {
      const key = localStorage.key(index);
      if (key) {
        values[key] = localStorage.getItem(key);
      }
    }
    return JSON.stringify(values);
  });
}

function pagination(count: number) {
  return { limit: 20, offset: 0, count, has_more: false };
}

function overviewDTO() {
  return {
    generated_at: NOW,
    cards: [
      {
        id: "api",
        title: "API",
        status: "ok",
        summary: "HTTP intake is healthy.",
        metrics: [{ label: "status", value: "ready", status: "ok" }],
        raw_prompt: RAW_PROMPT,
      },
      {
        id: "payments",
        title: "Payments",
        status: "warning",
        summary: "One stale payment needs operator review.",
        metrics: [{ label: "stale", value: "1", status: "warning" }],
        raw_payload: RAW_PAYMENT_PAYLOAD,
      },
    ],
  };
}

function queueDTO() {
  return {
    generated_at: NOW,
    degradation_state: "normal",
    backlog: [
      { label: "provider_submit", value: "1", status: "ok" },
      { label: "delivery", value: "0", status: "ok" },
    ],
    oldest_queued_age_seconds: 12,
    retry_count: 0,
    dlq: { status: "not_wired", reason: "DLQ backend aggregation is not wired." },
    provider_circuit: { status: "not_wired", reason: "Circuit backend aggregation is not wired." },
  };
}

function jobsDTO() {
  return {
    generated_at: NOW,
    items: [
      {
        lookup_id: "lookup_stage9_job",
        display_id: "job_stage9_display",
        correlation_ref: "corr_stage9_safe",
        operation: "image_generate",
        modality: "image",
        status: "failed_retryable",
        error_class: "delivery_timeout",
        cost_estimate: 10,
        cost_reserved: 10,
        cost_captured: 0,
        input_count: 1,
        output_count: 0,
        created_at: NOW,
        updated_at: NOW,
        age_seconds: 30,
        prompt: RAW_PROMPT,
        private_artifact_url: PRIVATE_ARTIFACT_URL,
      },
    ],
    pagination: pagination(1),
    queue: queueDTO(),
  };
}

function jobDetailDTO() {
  return {
    job: jobsDTO().items[0],
    allowed_next_statuses: ["queued"],
    artifacts: {
      input_refs: ["artifact_input_safe"],
      output_refs: [],
    },
    reservation: {
      status: "reserved",
      amount: 10,
      expires_at: NOW,
      updated_at: NOW,
    },
    delivery: {
      status: "failed_retryable",
      attempts: 1,
      retry_count: 1,
      last_error_class: "delivery_timeout",
      last_artifact_ref: "artifact_output_safe",
      last_delivery_type: "vk",
      last_delivery_status: "failed",
    },
    delivery_events: [
      {
        type: "vk",
        status: "failed",
        attempt_no: 1,
        error_class: "delivery_timeout",
        artifact_ref: "artifact_output_safe",
        created_at: NOW,
        updated_at: NOW,
        raw_stack: RAW_STACK,
      },
    ],
  };
}

function paymentsDTO() {
  return {
    generated_at: NOW,
    intents: [
      {
        display_id: "pay_stage9_pending",
        action_ref: "opact_v1_stage9_pending_action",
        user_ref: "user_stage9_safe",
        status: "provider_pending",
        amount: 9900,
        currency: "RUB",
        credits: 100,
        provider: "mock",
        provider_payment_ref: "provider_payment_safe",
        confirmation_state: "available",
        capture_state: "open",
        cancel_state: "cancelable_by_operator_endpoint",
        refund_state: "unavailable",
        stale: true,
        stale_seconds: 600,
        created_at: NOW,
        updated_at: NOW,
        raw_provider_payload: RAW_PAYMENT_PAYLOAD,
      },
      {
        display_id: "pay_stage9_succeeded",
        action_ref: ACTION_REF,
        user_ref: "user_stage9_safe",
        status: "succeeded",
        amount: 9900,
        currency: "RUB",
        credits: 100,
        provider: "mock",
        provider_payment_ref: "provider_payment_safe_2",
        confirmation_state: "none",
        capture_state: "captured",
        cancel_state: "terminal",
        refund_state: "eligible_policy_check_required",
        stale: false,
        created_at: NOW,
        updated_at: NOW,
      },
    ],
    events: [
      {
        display_id: "event_stage9_safe",
        provider: "mock",
        event_type: "payment.succeeded",
        provider_payment_ref: "provider_payment_safe_2",
        processed: true,
        processed_at: NOW,
        received_at: NOW,
        updated_at: NOW,
        raw_payload: RAW_PAYMENT_PAYLOAD,
      },
    ],
    refunds: [],
    reconciliation: {
      status: "needs_attention",
      pending_count: 1,
      stale_count: 1,
      unprocessed_event_count: 0,
      refund_count: 0,
      stale_after_seconds: 300,
    },
    pagination: pagination(2),
  };
}

function providersDTO() {
  return {
    generated_at: NOW,
    providers: [
      {
        provider_class: "deepinfra",
        model_class: "video",
        modality: "video",
        health: "ok",
        circuit_state: "ok",
        rate_limit_count: 0,
        provider_failed_count: 0,
        invalid_output_count: 0,
        fallback_state: "ready",
        contract_configured: true,
        quality_guard_enabled: true,
        source: "runtime_config_snapshot",
        raw_model_id: RAW_PROVIDER_PAYLOAD,
      },
    ],
    fallback: { status: "ok", provider_classes: ["deepinfra", "mock"], summary: "Fallback is configured." },
    provider_waste: {
      id: "provider_waste",
      title: "Provider waste",
      status: "ok",
      value: "0",
      source: "metrics",
      summary: "No waste detected.",
    },
    delivery_capture_gap: {
      id: "delivery_capture_gap",
      title: "Delivery capture gap",
      status: "ok",
      value: "0",
      source: "metrics",
      summary: "No capture gap detected.",
    },
    circuit: { status: "not_wired", source: "runtime", summary: "Automatic disable remains disabled." },
  };
}

function mediaSafetyDTO() {
  return {
    generated_at: NOW,
    policy: {
      pipeline_enabled: true,
      probe_policy: "probe_required",
      transcode_policy: "never",
      raw_provider_video_policy: "if_probe_passed",
      reference_uploads_enabled: true,
      webp_reference_enabled: false,
      max_image_upload_bytes: 5242880,
      max_image_pixels: 12000000,
      max_video_size_bytes: 52428800,
      max_video_duration_sec: 30,
      max_concurrent_uploads: 8,
      max_concurrent_probes: 4,
      max_concurrent_transcodes: 0,
      max_pending_variants: 100,
      queue_degrade_threshold: 1000,
      provider_max_attempts_per_job: 1,
      provider_fallback_budget_per_job: 0,
      provider_quality_guard_enabled: true,
      provider_quality_degraded_failures: 5,
      provider_quality_disabled_failures: 10,
    },
    uploads: [
      {
        id: "upload_rejects",
        title: "Upload rejects",
        status: "ok",
        value: "0",
        source: "metrics",
        summary: "No upload rejection spike.",
      },
    ],
    queue: queueDTO(),
    processing: [
      {
        id: "fast_path",
        title: "Video fast path",
        status: "ok",
        value: "enabled",
        source: "policy",
        summary: "Safe provider video skips transcode.",
      },
    ],
    cleanup: {
      id: "cleanup",
      title: "Cleanup",
      status: "ok",
      value: "0",
      source: "maintenance",
      summary: "No cleanup backlog.",
    },
  };
}
