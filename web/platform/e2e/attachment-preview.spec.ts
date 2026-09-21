import { expect, test, type Page } from "@playwright/test";

test.skip(process.env.NEIROHUB_LOCAL_WORKSPACE_PREVIEW !== "1", "Requires local preview; upload responses are isolated test fixtures.");
test.beforeEach(async ({ page }) => {
  // These tests deliberately exercise the real XHR/fetch boundary with routed responses.
  await page.addInitScript(() => sessionStorage.setItem("neirohub.local-scenarios.v1", JSON.stringify({ enabled: false, upload: "success", quote: "success" })));
});

async function openComposer(page: Page, locale = "ru") {
  await page.goto(`/${locale}/app/chat/20000000-0000-4000-8000-000000000004`);
  await page.locator('[data-variant="composer"] > button').click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("searchbox").fill("Seedream 5.0 Pro");
  await dialog.getByRole("button", { name: /Seedream 5.0 Pro/ }).first().click();
  await expect(dialog).toBeHidden();
  await page.locator("textarea").fill("Attachment preview check");
}

async function dropPhoto(page: Page, name = "preview.png", path = "/assets/images/inspiration/paper-crane-cloud.png") {
  await page.evaluate(async ({ name, path }) => {
    const blob = await (await fetch(path)).blob();
    const transfer = new DataTransfer();
    transfer.items.add(new File([blob], name, { type: "image/png" }));
    document.querySelector('[data-ui="workspace-file-drop"]')!.dispatchEvent(new DragEvent("drop", { bubbles: true, dataTransfer: transfer }));
  }, { name, path });
}

const uploadResult = { artifact_id: "70000000-0000-4000-8000-000000000009", mime_type: "image/png", size_bytes: 2358724, width: 1024, height: 1536 };

test("photo tile supports loading, hover tooltip, keyboard removal, failure and retry", async ({ page, context, baseURL }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await context.addCookies([{ name: "nh_csrf", value: "local-test-only", url: baseURL! }]);
  let finish: (status: number) => void = () => {};
  let uploads = 0;
  await page.route("**/web/v1/input-artifacts?*", async route => {
    uploads++;
    const status = await new Promise<number>(resolve => { finish = resolve; });
    await route.fulfill({ status, json: status === 200 ? uploadResult : { error: "unavailable" } });
  });
  await page.route("**/web/v1/image-reference-quote?*", route => route.fulfill({ json: { credits: 20 } }));
  await openComposer(page);
  await dropPhoto(page);
  await expect.poll(() => uploads).toBe(1);
  const tile = page.locator('[data-ui="chat-attachment"]');
  const image = tile.getByRole("img", { name: "preview.png" });
  await expect(image).toBeVisible();
  await expect(tile.getByRole("progressbar")).toBeVisible();
  await expect(page.getByRole("button", { name: "Отправить", exact: true })).toBeDisabled();
  const imageBox = await image.boundingBox(); const fieldBox = await page.locator("textarea").boundingBox();
  expect(imageBox!.y + imageBox!.height).toBeLessThan(fieldBox!.y);
  await page.screenshot({ path: "test-results/attachment-loading-dark.png" });
  finish(200);
  await expect(tile).toHaveAttribute("data-status", "ready");
  await expect(tile.getByRole("progressbar")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Отправить", exact: true })).toBeEnabled();
  const remove = tile.getByRole("button", { name: "Удалить файл: preview.png", exact: true });
  const removeWrapper = remove.locator("../..");
  await page.mouse.move(10, 10);
  await expect(removeWrapper).toHaveCSS("opacity", "0");
  await tile.getByRole("button", { name: "Просмотр файла: preview.png", exact: true }).hover();
  await expect(removeWrapper).toHaveCSS("opacity", "1");
  await remove.hover();
  await expect(tile.getByRole("tooltip", { name: "Удалить файл", exact: true })).toBeVisible();
  await expect(image).toHaveCSS("object-fit", "cover");
  await expect(image).toHaveCSS("width", "180px");
  await expect(image).toHaveCSS("height", "180px");
  await expect(image.locator("..")).toHaveCSS("border-width", "0px");
  await expect(image.locator("..")).toHaveCSS("background-color", "rgba(0, 0, 0, 0)");
  await expect(image.locator("..")).toHaveCSS("border-radius", "8px");
  await page.screenshot({ path: "test-results/attachment-hover-dark.png" });
  await page.emulateMedia({ colorScheme: "light" });
  await page.screenshot({ path: "test-results/attachment-hover-light.png" });
  await page.mouse.move(10, 10);
  await remove.focus(); await expect(removeWrapper).toHaveCSS("opacity", "1");
  await page.keyboard.press("Enter"); await expect(tile).toHaveCount(0);
  await dropPhoto(page); await expect.poll(() => uploads).toBe(2);
  finish(503);
  await expect(tile).toHaveAttribute("data-status", "failed");
  await expect(tile.getByRole("progressbar")).toHaveCount(0);
  const retry = tile.getByRole("button", { name: "Повторить загрузку: preview.png", exact: true });
  await retry.hover();
  await expect(tile.getByRole("tooltip", { name: "Повторить", exact: true })).toBeVisible();
  await expect(retry).toHaveCSS("background-color", "rgba(0, 0, 0, 0)");
  await expect(tile).toHaveCSS("height", "180px");
  await page.emulateMedia({ colorScheme: "dark" });
  await page.screenshot({ path: "test-results/attachment-failure-overlay-dark.png", animations: "disabled" });
  await retry.click();
  await expect.poll(() => uploads).toBe(3);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await expect(tile.getByRole("progressbar")).toHaveCSS("animation-name", "none");
  await remove.click();
  await expect(tile).toHaveCount(0);
  finish(200);
  await expect(page.locator("textarea")).toHaveValue("Attachment preview check");
});

test.describe("touch attachments", () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true, colorScheme: "light" });
  test("keeps deletion visible without hover and uses the selected language", async ({ page, context, baseURL }) => {
    await context.addCookies([{ name: "nh_csrf", value: "local-test-only", url: baseURL! }]);
    let uploads = 0;
    await page.route("**/web/v1/input-artifacts?*", route => {
      const number = ++uploads;
      return route.fulfill({ status: number === 3 ? 503 : 200, json: { ...uploadResult, artifact_id: `70000000-0000-4000-8000-${String(number).padStart(12, "0")}` } });
    });
    await page.route("**/web/v1/image-reference-quote?*", route => route.fulfill({ json: { credits: 20 } }));
    await openComposer(page, "en"); await dropPhoto(page);
    const tile = page.locator('[data-ui="chat-attachment"]');
    await expect(tile).toHaveAttribute("data-status", "ready");
    const firstImage = page.getByRole("img", { name: "preview.png", exact: true });
    const previewURL = await firstImage.getAttribute("src");
    await expect(firstImage).toHaveCSS("width", "180px");
    const remove = tile.getByRole("button", { name: "Remove file: preview.png", exact: true });
    await expect(remove.locator("../..")).toHaveCSS("opacity", "1");
    const input = page.locator('[data-ui="input-surface"]').filter({ has: page.locator("textarea") });
    const box = await input.boundingBox();
    expect(box!.x + box!.width).toBeLessThanOrEqual(390);
    await page.screenshot({ path: "test-results/attachment-mobile-light.png" });
    await dropPhoto(page, "second.png", "/assets/images/models/default-model-87465de8.png");
    await dropPhoto(page, "third.png", "/assets/images/workspace/neirohub-how-it-works-poster.png");
    await expect(tile).toHaveCount(3);
    for (const image of await tile.getByRole("img").all()) {
      await expect(image).toHaveCSS("width", "72px");
      await expect(image).toHaveCSS("height", "72px");
      await expect(image).toHaveCSS("object-fit", "cover");
    }
    const boxes = await tile.evaluateAll(elements => elements.map(element => ({ x: element.getBoundingClientRect().x, y: element.getBoundingClientRect().y })));
    expect(boxes[1].y).toBe(boxes[0].y);
    expect(boxes[2].y).toBe(boxes[0].y);
    await page.screenshot({ path: "test-results/attachment-multiple-mobile.png" });
    const failed = tile.filter({ has: page.getByRole("img", { name: "third.png", exact: true }) });
    await expect(failed).toHaveAttribute("data-status", "failed");
    const retry = failed.getByRole("button", { name: "Retry upload: third.png", exact: true });
    const retryBox = (await retry.boundingBox())!;
    const removeBox = (await failed.getByRole("button", { name: "Remove file: third.png", exact: true }).boundingBox())!;
    expect(retryBox.y).toBeGreaterThanOrEqual(removeBox.y + removeBox.height);
    await retry.tap();
    await expect(failed).toHaveAttribute("data-status", "ready");
    await page.getByRole("button", { name: "Remove file: second.png", exact: true }).tap();
    await expect(firstImage).toHaveCSS("width", "72px");
    await page.getByRole("button", { name: "Remove file: third.png", exact: true }).tap();
    await expect(firstImage).toHaveCSS("width", "180px");
    await expect(firstImage).toHaveAttribute("src", previewURL!);
    expect(uploads).toBe(4);
    await remove.tap(); await expect(tile).toHaveCount(0);
    await expect(page.locator("textarea")).toHaveValue("Attachment preview check");
  });
});

test("duplicate files show a notice for drop, picker and paste without uploading twice", async ({ page, context, baseURL }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await context.addCookies([{ name: "nh_csrf", value: "local-test-only", url: baseURL! }]);
  let uploads = 0;
  await page.route("**/web/v1/input-artifacts?*", route => {
    uploads++;
    return route.fulfill({ json: uploadResult });
  });
  await page.route("**/web/v1/image-reference-quote?*", route => route.fulfill({ json: { credits: 20 } }));
  await openComposer(page);
  await dropPhoto(page);
  const tile = page.locator('[data-ui="chat-attachment"]');
  await expect(tile).toHaveAttribute("data-status", "ready");
  const notice = page.getByRole("alertdialog", { name: "Этот файл уже прикреплён" });
  await dropPhoto(page, "renamed.png");
  await expect(notice).toBeVisible();
  await expect(notice.getByRole("button", { name: "Понятно" })).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(notice.getByRole("button", { name: "Понятно" })).toBeFocused();
  await page.screenshot({ path: "test-results/attachment-duplicate-dark.png", animations: "disabled" });
  await notice.getByRole("button", { name: "Понятно" }).click();
  await expect(notice).toBeHidden();
  await page.locator('input[type="file"]').first().setInputFiles("public/assets/images/inspiration/paper-crane-cloud.png");
  await expect(notice).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(notice).toBeHidden();
  await page.locator("textarea").focus();
  await page.evaluate(async () => {
    const blob = await (await fetch("/assets/images/inspiration/paper-crane-cloud.png")).blob();
    const transfer = new DataTransfer();
    transfer.items.add(new File([blob], "clipboard.png", { type: "image/png" }));
    document.querySelector("textarea")!.dispatchEvent(new ClipboardEvent("paste", { bubbles: true, clipboardData: transfer }));
  });
  await expect(notice).toBeVisible();
  await page.emulateMedia({ reducedMotion: "reduce", colorScheme: "light" });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: "test-results/attachment-duplicate-mobile-light.png" });
  await notice.getByRole("button", { name: "Понятно" }).click();
  await expect(notice).toBeHidden();
  await expect(page.locator("textarea")).toBeFocused();
  await expect(page.locator("textarea")).toHaveValue("Attachment preview check");
  await expect(tile).toHaveCount(1);
  expect(uploads).toBe(1);
  await tile.getByRole("button", { name: "Просмотр файла: preview.png", exact: true }).hover();
  await tile.getByRole("button", { name: "Удалить файл: preview.png" }).click();
  await dropPhoto(page);
  await expect(tile).toHaveAttribute("data-status", "ready");
  expect(uploads).toBe(2);
  await expect(notice).toBeHidden();
});

test("cost appears only when ready and quote failures follow a completed upload", async ({ page, context, baseURL }) => {
  const alerts = page.locator("form").getByRole("alert");
  await page.emulateMedia({ colorScheme: "dark" });
  await context.addCookies([{ name: "nh_csrf", value: "local-test-only", url: baseURL! }]);
  let finishUpload: (status: number) => void = () => {};
  let finishQuote: (status: number) => void = () => {};
  let uploads = 0;
  let quotes = 0;
  await page.route("**/web/v1/input-artifacts?*", async route => {
    uploads++;
    const status = await new Promise<number>(resolve => { finishUpload = resolve; });
    await route.fulfill({ status, json: status === 200 ? uploadResult : { error: "unavailable" } });
  });
  await page.route("**/web/v1/image-reference-quote?*", async route => {
    quotes++;
    const status = await new Promise<number>(resolve => { finishQuote = resolve; });
    await route.fulfill({ status, json: status === 200 ? { credits: 37 } : { error: "unavailable" } });
  });
  await page.goto("/ru/app");
  await expect(page.getByText(/Стоимость: —/)).toHaveCount(0);
  await expect(alerts).toHaveCount(0);
  await openComposer(page);
  expect(quotes).toBe(0);
  await dropPhoto(page);
  await expect.poll(() => uploads).toBe(1);
  const tile = page.locator('[data-ui="chat-attachment"]');
  const submit = page.getByRole("button", { name: "Отправить", exact: true });
  await expect(tile.getByRole("progressbar")).toBeVisible();
  await expect(page.getByText(/^Стоимость:/)).toHaveCount(0);
  expect(quotes).toBe(0);
  finishUpload(503);
  await expect(tile).toHaveAttribute("data-status", "failed");
  expect(quotes).toBe(0);
  await tile.getByRole("button", { name: "Повторить загрузку: preview.png", exact: true }).click();
  await expect.poll(() => uploads).toBe(2);
  finishUpload(200);
  await expect.poll(() => quotes).toBe(1);
  await expect(page.getByText(/^Стоимость:/)).toHaveCount(0);
  await expect(submit).toBeDisabled();
  finishQuote(503);
  const retry = page.getByRole("button", { name: "Повторить расчёт" });
  await expect(alerts).toHaveText("Стоимость пока недоступна. Попробуйте рассчитать её ещё раз.");
  await expect(retry).toHaveCSS("background-color", "rgba(0, 0, 0, 0)");
  await page.screenshot({ path: "test-results/reference-quote-failure-dark.png", animations: "disabled" });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ colorScheme: "light" });
  await retry.scrollIntoViewIfNeeded();
  const noticeBox = await retry.locator('xpath=ancestor::*[@data-ui="popover-panel"]').boundingBox();
  expect(noticeBox!.x).toBeGreaterThanOrEqual(0);
  expect(noticeBox!.x + noticeBox!.width).toBeLessThanOrEqual(390);
  await page.screenshot({ path: "test-results/reference-quote-failure-mobile-light.png", animations: "disabled" });
  await retry.click();
  await expect.poll(() => quotes).toBe(2);
  await expect(alerts).toHaveCount(0);
  await expect(submit).toBeDisabled();
  finishQuote(200);
  await expect(page.getByLabel("Стоимость: 37 звёзд", { exact: true })).toBeVisible();
  await expect(page.getByLabel("Стоимость: 37 звёзд", { exact: true }).getByTestId("credit-star-icon")).toBeVisible();
  await page.screenshot({ path: "test-results/composer-cost-star.png", animations: "disabled" });
  await expect(submit).toBeEnabled();
  await expect(page.locator("textarea")).toHaveValue("Attachment preview check");
  await tile.getByRole("button", { name: "Просмотр файла: preview.png", exact: true }).hover();
  await tile.getByRole("button", { name: "Удалить файл: preview.png" }).click();
  await expect(page.getByLabel("Стоимость: 37 звёзд", { exact: true })).toHaveCount(0);
  await expect(alerts).toHaveCount(0);
  await page.reload();
  await expect(alerts).toHaveCount(0);
  await expect(page.getByText(/Стоимость: —/)).toHaveCount(0);
  expect(quotes).toBe(2);
});
