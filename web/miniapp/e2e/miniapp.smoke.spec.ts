import { Buffer } from "node:buffer";
import { expect, test, type Page, type Route } from "@playwright/test";

const LAUNCH_MARKER = "stage4_launch_marker_private";
const PRIVATE_STORAGE_URL = "https://private-storage.example.test/artifacts/raw.png";
const CHAT_PROMPT = "stage4 chat prompt must stay out of telemetry";
const IMAGE_PROMPT = "stage4 image prompt must stay out of telemetry";
const TEXT_ARTIFACT_ID = "11111111-1111-4111-8111-111111111111";
const IMAGE_ARTIFACT_ID = "22222222-2222-4222-8222-222222222222";
const CHAT_JOB_ID = "33333333-3333-4333-8333-333333333333";
const IMAGE_JOB_ID = "44444444-4444-4444-8444-444444444444";
const SAVED_CHAT_JOB_ID = "77777777-7777-4777-8777-777777777777";
const NOW = "2026-06-12T10:00:00Z";
const SAVED_USER_MESSAGE = "Saved default chat question";
const SAVED_BOT_MESSAGE = "Saved default chat answer";

const onePixelPng = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=",
  "base64",
);

type MockMode = "happy" | "errors";

type MiniappMocks = {
  telemetryBodies: string[];
  consoleMessages: string[];
  requestedPaths: string[];
};

function pagination(count = 0) {
  return { limit: 50, offset: 0, count, has_more: false };
}

function job(overrides: Record<string, unknown>) {
  return {
    id: CHAT_JOB_ID,
    operation: "text_generate",
    modality: "text",
    status: "received",
    prompt: CHAT_PROMPT,
    conversation_id: "default",
    cost_estimate: 1,
    cost_captured: 0,
    output_artifact_ids: [],
    created_at: NOW,
    updated_at: NOW,
    ...overrides,
  };
}

async function json(route: Route, body: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

async function setupMiniappMocks(page: Page, mode: MockMode): Promise<MiniappMocks> {
  const state = {
    chatPolls: 0,
    imagePolls: 0,
  };
  const telemetryBodies: string[] = [];
  const consoleMessages: string[] = [];
  const requestedPaths: string[] = [];

  page.on("console", (message) => {
    consoleMessages.push(message.text());
  });

  await page.route("**/miniapp/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method();
    requestedPaths.push(`${method} ${path}`);

    if (path === "/miniapp/client-events") {
      telemetryBodies.push(request.postData() ?? "");
      await route.fulfill({ status: 204 });
      return;
    }

    if (mode === "errors") {
      if (path === "/miniapp/chat/messages" && method === "POST") {
        await json(route, { error: "rate_limited" }, 429);
        return;
      }
      await json(route, { error: path === "/miniapp/balance" ? "auth_error" : "service_unavailable" }, path === "/miniapp/balance" ? 401 : 503);
      return;
    }

    if (path === "/miniapp/balance") {
      await json(route, { balance_credits: 250 });
      return;
    }

    if (path === "/miniapp/referral") {
      await json(route, {
        code: "ABCD2345",
        invite_url: "https://vk.com/app123#ref=ABCD2345",
        invited_count: 2,
        registered_count: 2,
        activated_count: 1,
        rewarded_count: 1,
        referrer_signup_reward_credits: 10,
        referred_signup_reward_credits: 10,
      });
      return;
    }

    if (path === "/miniapp/payment-products") {
      await json(route, {
        items: [
          {
            id: "product-basic",
            code: "basic",
            title: "Basic credits",
            amount: 19900,
            currency: "RUB",
            credits: 100,
            price_version: 1,
          },
        ],
        pagination: pagination(1),
      });
      return;
    }

    if (path === "/miniapp/model-catalog") {
      await json(route, {
        items: [
          {
            type: "image",
            id: "nano_banana_pro",
            name: "Nano Banana Pro",
            description: "High-detail image model",
            estimate_credits: 10,
            enabled: true,
            quality_options: ["standard"],
            default_quality: "standard",
            requires_start_image: false,
            supports_reference_image: true,
            max_reference_images: 16,
          },
          {
            type: "video",
            id: "video_kling_o3_standard",
            alias: "video_kling_o3_standard",
            name: "Kling O3 Standard",
            description: "Video route",
            estimate_credits: 50,
            enabled: true,
            allowed_durations_sec: [5, 10],
            default_duration_sec: 5,
            requires_start_image: false,
            supports_reference_image: false,
          },
        ],
      });
      return;
    }

    if (path === "/miniapp/payments" && method === "GET") {
      await json(route, {
        items: [
          {
            id: "payment-active",
            product_id: "product-basic",
            status: "waiting_for_user",
            amount: 19900,
            currency: "RUB",
            credits: 100,
            price_version: 1,
            confirmation_url: "https://pay.example.test/continue/stage4-intent",
            reused_active_payment: false,
            created_at: NOW,
            updated_at: NOW,
          },
        ],
        pagination: pagination(1),
      });
      return;
    }

    if (path === "/miniapp/jobs" && method === "GET") {
      await json(route, {
        items: [
          job({
            id: IMAGE_JOB_ID,
            operation: "image_generate",
            modality: "image",
            status: "succeeded",
            prompt: IMAGE_PROMPT,
            conversation_id: undefined,
            output_artifact_ids: [IMAGE_ARTIFACT_ID],
          }),
          job({
            id: CHAT_JOB_ID,
            operation: "text_generate",
            modality: "text",
            status: "succeeded",
            prompt: CHAT_PROMPT,
          }),
        ],
        pagination: pagination(2),
      });
      return;
    }

    if (path === "/miniapp/chat/messages" && method === "GET") {
      const items = [
        {
          id: "55555555-5555-4555-8555-555555555555",
          job_id: SAVED_CHAT_JOB_ID,
          seq: 1,
          role: "user",
          text: SAVED_USER_MESSAGE,
          created_at: NOW,
        },
        {
          id: "66666666-6666-4666-8666-666666666666",
          job_id: SAVED_CHAT_JOB_ID,
          seq: 2,
          role: "bot",
          text: SAVED_BOT_MESSAGE,
          created_at: NOW,
        },
      ];
      if (state.chatPolls > 1) {
        items.push(
          {
            id: "88888888-8888-4888-8888-888888888888",
            job_id: CHAT_JOB_ID,
            seq: 3,
            role: "user",
            text: CHAT_PROMPT,
            created_at: NOW,
          },
          {
            id: "99999999-9999-4999-8999-999999999999",
            job_id: CHAT_JOB_ID,
            seq: 4,
            role: "bot",
            text: "Mocked safe assistant answer",
            created_at: NOW,
          },
        );
      }
      await json(route, {
        items,
        pagination: pagination(items.length),
      });
      return;
    }

    if (path === "/miniapp/chat/messages" && method === "POST") {
      await json(route, job({ status: "provider_processing" }));
      return;
    }

    if (path === "/miniapp/jobs" && method === "POST") {
      await json(route, job({
        id: IMAGE_JOB_ID,
        operation: "image_generate",
        modality: "image",
        status: "provider_processing",
        prompt: IMAGE_PROMPT,
        conversation_id: undefined,
      }));
      return;
    }

    if (path === "/miniapp/estimate" && method === "POST") {
      await json(route, {
        operation: "image_generate",
        model_id: "nano_banana_pro",
        model_name: "Nano Banana Pro",
        cost_estimate: 10,
        balance_credits: 250,
        enough_credits: true,
      });
      return;
    }

    if (path === `/miniapp/jobs/${CHAT_JOB_ID}`) {
      state.chatPolls += 1;
      await json(
        route,
        job({
          status: state.chatPolls > 1 ? "succeeded" : "provider_processing",
          output_artifact_ids: state.chatPolls > 1 ? [TEXT_ARTIFACT_ID] : [],
        }),
      );
      return;
    }

    if (path === `/miniapp/jobs/${IMAGE_JOB_ID}`) {
      state.imagePolls += 1;
      await json(
        route,
        job({
          id: IMAGE_JOB_ID,
          operation: "image_generate",
          modality: "image",
          status: state.imagePolls > 1 ? "succeeded" : "provider_processing",
          prompt: IMAGE_PROMPT,
          conversation_id: undefined,
          output_artifact_ids: state.imagePolls > 1 ? [IMAGE_ARTIFACT_ID] : [],
        }),
      );
      return;
    }

    if (path === `/miniapp/artifacts/${TEXT_ARTIFACT_ID}`) {
      await route.fulfill({
        status: 200,
        contentType: "text/plain; charset=utf-8",
        body: "Mocked safe assistant answer",
      });
      return;
    }

    if (path === `/miniapp/artifacts/${IMAGE_ARTIFACT_ID}`) {
      await route.fulfill({
        status: 200,
        contentType: "image/png",
        body: onePixelPng,
      });
      return;
    }

    await json(route, { error: "not_found" }, 404);
  });

  return { telemetryBodies, consoleMessages, requestedPaths };
}

async function openMiniapp(page: Page) {
  const launchParams = new URLSearchParams({
    vk_user_id: "42",
    vk_ts: "1781200000",
    vk_platform: "mobile_web",
    sign: LAUNCH_MARKER,
    private_artifact_url: PRIVATE_STORAGE_URL,
  });
  await page.goto(`/?${launchParams.toString()}`);
  await expect(page.locator(".chat")).toBeVisible();
}

async function expectNoHorizontalOverflow(page: Page) {
  const overflow = await page.evaluate(() => ({
    html: document.documentElement.scrollWidth - document.documentElement.clientWidth,
    body: document.body.scrollWidth - document.body.clientWidth,
  }));
  expect(overflow.html).toBeLessThanOrEqual(1);
  expect(overflow.body).toBeLessThanOrEqual(1);
}

async function expectNoSensitiveLeaks(page: Page, mocks: MiniappMocks) {
  const bodyText = await page.locator("body").innerText();
  expect(bodyText).not.toContain(LAUNCH_MARKER);
  expect(bodyText).not.toContain(PRIVATE_STORAGE_URL);

  const mediaSources = await page.locator("img, video").evaluateAll((nodes) =>
    nodes.map((node) => (node as HTMLImageElement | HTMLVideoElement).src).filter(Boolean),
  );
  for (const src of mediaSources) {
    expect(src).not.toContain(LAUNCH_MARKER);
    expect(src).not.toContain(PRIVATE_STORAGE_URL);
    expect(src).not.toContain("vk_user_id");
    expect(src).not.toContain("sign=");
  }

  for (const body of mocks.telemetryBodies) {
    expect(body).not.toContain(LAUNCH_MARKER);
    expect(body).not.toContain(PRIVATE_STORAGE_URL);
    expect(body).not.toContain(CHAT_PROMPT);
    expect(body).not.toContain(IMAGE_PROMPT);
    expect(body).not.toContain("vk_user_id");
    expect(body).not.toContain("cookie");
  }

  for (const message of mocks.consoleMessages) {
    expect(message).not.toContain(LAUNCH_MARKER);
    expect(message).not.toContain(PRIVATE_STORAGE_URL);
    expect(message).not.toContain(CHAT_PROMPT);
    expect(message).not.toContain(IMAGE_PROMPT);
  }
}

async function expectNoMultiChatControls(page: Page) {
  await expect(page.getByRole("button", { name: "История диалогов" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Новый диалог" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Удалить диалог" })).toHaveCount(0);
  await expect(page.locator(".drawer, .drawer-overlay, .chat-item")).toHaveCount(0);
}

test.describe("VK Mini App smoke", () => {
  test("renders mobile app, completes mocked chat and media flows safely", async ({ page }) => {
    const mocks = await setupMiniappMocks(page, "happy");

    await openMiniapp(page);
    await expectNoHorizontalOverflow(page);
    await expectNoMultiChatControls(page);
    await expect(page.locator(".msg--user")).toContainText(SAVED_USER_MESSAGE);
    await expect(page.locator(".msg--bot .bubble")).toContainText(SAVED_BOT_MESSAGE);

    await page.locator(".composer textarea").fill(CHAT_PROMPT);
    await page.locator(".composer__send").click();
    await expect(page.locator(".msg--user").filter({ hasText: CHAT_PROMPT })).toHaveCount(1);
    await expect(page.locator(".msg--bot .bubble").filter({ hasText: "Mocked safe assistant answer" })).toHaveCount(
      1,
      { timeout: 8_000 },
    );

    await page.locator(".nh-tabbar__btn", { hasText: "Создать" }).click();
    await expect(page.locator(".workflow-screen")).toBeVisible();
    await expect(page.locator(".create-model-trigger")).toContainText("Nano Banana Pro");
    await page.getByRole("tab", { name: "Видео" }).click();
    await expect(page.locator(".create-model-trigger")).toContainText("Kling O3 Standard");
    await expect(page.getByRole("group", { name: "Длительность видео" })).toBeVisible();
    await page.getByRole("tab", { name: "Фото" }).click();
    await expect(page.locator(".create-model-trigger")).toContainText("Nano Banana Pro");
    await page.locator("#workflow-prompt").fill(IMAGE_PROMPT);
    await expect(page.locator(".create-prompt__send")).toBeEnabled({ timeout: 8_000 });
    await page.locator(".create-prompt__send").click();
    await expect(page.locator(".media-result__media")).toBeVisible({ timeout: 10_000 });
    const mediaSrc = await page.locator(".media-result__media").getAttribute("src");
    expect(mediaSrc).toMatch(/^blob:/);

    await page.locator(".nh-tabbar__btn", { hasText: "Профиль" }).click();
    await expect(page.locator(".settings-balance-hero strong")).not.toHaveText("...");
    await expect(page.locator(".payment-pending")).toBeVisible();
    await page.locator(".settings-history-toggle").click();
    await expect(page.locator(".settings-history-row")).toHaveCount(1);
    await expect(page.locator(".settings-history-row")).toContainText(IMAGE_PROMPT);
    await expect(page.locator(".settings-history-row")).not.toContainText(CHAT_PROMPT);
    await expect(page.getByRole("button", { name: "Диалоги" })).toHaveCount(0);

    await expectNoHorizontalOverflow(page);
    await expectNoSensitiveLeaks(page, mocks);
    expect(mocks.requestedPaths.some((item) => item.includes("/miniapp/chat/conversations"))).toBe(false);
  });

  test("keeps a usable safe screen on auth, rate-limit and API errors", async ({ page }) => {
    const mocks = await setupMiniappMocks(page, "errors");

    await openMiniapp(page);
    await expectNoHorizontalOverflow(page);
    await expectNoMultiChatControls(page);

    await page.locator(".composer textarea").fill("rate limited smoke prompt");
    await page.locator(".composer__send").click();
    await expect(page.locator(".bubble__err")).toBeVisible();

    await page.locator(".nh-tabbar__btn", { hasText: "Профиль" }).click();
    await expect(page.locator(".settings-screen")).toBeVisible();
    await expect(page.locator(".settings-notice, .settings-empty").first()).toBeVisible();

    await expectNoHorizontalOverflow(page);
    await expectNoSensitiveLeaks(page, mocks);
  });
});
