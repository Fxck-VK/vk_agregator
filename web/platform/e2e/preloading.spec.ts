import { expect, test } from "@playwright/test";

test.skip(process.env.NEIROHUB_LOCAL_WORKSPACE_PREVIEW !== "1", "Isolated local preview only.");
test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => sessionStorage.setItem("neirohub.local-scenarios.v1", JSON.stringify({ enabled: false })));
});

test("selected model is in server HTML before JavaScript runs", async ({ browser, baseURL }) => {
  const context = await browser.newContext({ javaScriptEnabled: false, baseURL });
  try {
    const page = await context.newPage();
    await page.goto("/ru/app/chats?model=seedream_5_0_lite");
    await expect(page.locator('header[data-testid="workspace-header"]')).toContainText("Seedream 5.0 Lite");
    await expect(page.locator("textarea")).toBeEnabled();
    await expect(page.locator('header[data-testid="workspace-header"]')).not.toContainText("Загружаем");
  } finally { await context.close(); }
});

test("slow history, failure and retry keep the same editable draft", async ({ page }) => {
  let release!: () => void;
  let reads = 0;
  let holding = true;
  const gate = new Promise<void>(resolve => { release = resolve; });
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.route("**/web/v1/conversations/*/messages?limit=100", async route => {
    reads++;
    if (holding) { await gate; await route.fulfill({ status: 503, body: "unavailable" }); }
    else await route.continue();
  });
  await page.goto("/ru/app/chat/20000000-0000-4000-8000-000000000004");
  await expect.poll(() => reads).toBeGreaterThanOrEqual(1);
  const input = page.locator("textarea");
  await expect(input).toBeEnabled();
  await input.fill("Черновик во время загрузки");
  await expect(page.locator('[data-ui="skeleton"]').first()).toBeVisible();
  const slowNotice = page.getByText("Загрузка занимает больше времени. Вы можете продолжать работать.");
  await expect(slowNotice).toBeVisible({ timeout: 6000 });
  const headerBounds = await page.locator('header[data-testid="workspace-header"]').boundingBox();
  const noticeBounds = await slowNotice.boundingBox();
  expect(noticeBounds!.y).toBeGreaterThanOrEqual(headerBounds!.y + headerBounds!.height);
  await page.screenshot({ path: ".tmp/preloading-history-slow.png" });
  release();
  holding = false;
  const retry = page.getByRole("button", { name: "Повторить", exact: true });
  await expect(retry).toBeVisible();
  await expect(input).toHaveValue("Черновик во время загрузки");
  await retry.click();
  await expect(page.locator('[data-ui="skeleton"]')).toHaveCount(0);
  await expect(input).toHaveValue("Черновик во время загрузки");
  await expect(input).toBeEnabled();
  expect(errors).toEqual([]);
});

test("cached files survive failed refresh and cards download only small previews", async ({ page }) => {
  let lists = 0;
  let results = 0;
  const images: string[] = [];
  page.on("request", request => {
    if (request.url().includes("/image-artifacts/")) images.push(request.url());
    if (/\/image-jobs\/.+\/result/.test(request.url())) results++;
  });
  await page.route("**/web/v1/image-jobs?*", route => {
    lists++;
    return lists === 1 ? route.continue() : route.fulfill({ status: 503, body: "unavailable" });
  });
  await page.goto("/ru/app/files?category=images");
  await expect(page.locator('[data-ui="media-image"][data-state="ready"]').first()).toBeVisible();
  const initialResults = results;
  expect(images.length).toBeGreaterThan(0);
  expect(images.every(url => url.endsWith("?preview=1"))).toBe(true);
  await page.locator('a[href="/ru/app/models"]').first().click();
  await expect(page).toHaveURL(/\/app\/models$/);
  await page.locator('a[href="/ru/app/files"]').first().click();
  await expect.poll(() => lists).toBe(2);
  await expect(page.locator('[data-ui="media-image"]').first()).toBeVisible();
  await expect(page.locator('[data-ui="state-notice"][role="alert"]')).toBeVisible();
  expect(results).toBe(initialResults);
});

test("losing connectivity retains the composer text and selected local photo", async ({ page, context, baseURL }) => {
  await context.addCookies([{ name: "nh_csrf", value: "local-test-only", url: baseURL! }]);
  await page.goto("/ru/app/chats?model=seedream_5_0_pro");
  await page.locator("textarea").fill("Мой запрос с фотографией");
  await context.setOffline(true);
  await page.evaluate(() => {
    const bytes = Uint8Array.from(atob("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jR1UAAAAASUVORK5CYII="), char => char.charCodeAt(0));
    const transfer = new DataTransfer();
    transfer.items.add(new File([bytes], "local.png", { type: "image/png" }));
    document.querySelector('[data-ui="workspace-file-drop"]')!.dispatchEvent(new DragEvent("drop", { bubbles: true, dataTransfer: transfer }));
  });
  await expect(page.locator('[data-ui="chat-attachment"]')).toHaveCount(1);
  await expect(page.locator("textarea")).toHaveValue("Мой запрос с фотографией");
  await expect(page.locator('[data-ui="chat-attachment"] img')).toHaveAttribute("src", /^blob:/);
  await context.setOffline(false);
});
