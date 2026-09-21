import { expect, test, type Page } from "@playwright/test";

test.skip(process.env.NEIROHUB_LOCAL_WORKSPACE_PREVIEW !== "1", "Requires local development preview.");

async function composer(page: Page, locale = "ru", newChat = false) {
  await page.goto(newChat ? `/${locale}/app/chats` : `/${locale}/app/chat/20000000-0000-4000-8000-000000000004`);
  await expect(page.locator('[data-ui="local-development-tools"]')).toBeVisible();
  if (newChat) await page.getByRole("banner").getByRole("button", { name: /Открыть список$/ }).click();
  else await page.locator('[data-variant="composer"] > button').click();
  const selector = page.getByRole("dialog");
  await selector.getByRole("searchbox").fill("Seedream 5.0 Pro");
  await selector.getByRole("button", { name: /Seedream 5.0 Pro/ }).first().click();
  await page.locator("textarea").fill("Local upload check");
}
async function drop(page: Page, name = "local.png", path = "/assets/images/inspiration/paper-crane-cloud.png") {
  await page.evaluate(async ({ name, path }) => {
    const blob = await (await fetch(path)).blob();
    const transfer = new DataTransfer();
    transfer.items.add(new File([blob], name, { type: "image/png" }));
    document.querySelector('[data-ui="workspace-file-drop"]')!.dispatchEvent(new DragEvent("drop", { bubbles: true, dataTransfer: transfer }));
  }, { name, path });
}
async function scenario(page: Page, name: string, group = "Загрузка фото") {
  await page.locator('[data-ui="local-development-tools"] button').click();
  await page.getByRole("toolbar", { name: group }).getByRole("button", { name, exact: true }).click();
  await page.getByRole("button", { name: "Закрыть локальную проверку" }).click();
}

for (const width of [1280, 390]) {
  test(`previews composer and sent photos in the shared viewer at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 844 });
    await page.emulateMedia({ colorScheme: "dark" });
    await composer(page);
    await drop(page);
    const tiles = page.locator('[data-ui="chat-attachment"]');
    await expect(tiles.first()).toHaveAttribute("data-status", "ready");
    const firstTrigger = tiles.first().getByRole("button", { name: "Просмотр файла: local.png" });
    await firstTrigger.click();
    const dialog = page.getByRole("dialog", { name: "Просмотр файла", exact: true });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole("complementary")).toHaveCount(0);
    await expect(dialog.getByRole("navigation")).toHaveCount(0);
    const previewImage = page.getByTestId("attachment-preview-preview").getByRole("img");
    await expect.poll(() => previewImage.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBeGreaterThan(0);
    const geometry = await previewImage.evaluate((img: HTMLImageElement) => {
      const rect = img.getBoundingClientRect();
      return { x: rect.x, y: rect.y, width: rect.width, height: rect.height, ratio: img.naturalWidth / img.naturalHeight };
    });
    expect(geometry.height).toBeGreaterThan(300);
    expect(geometry.x).toBeGreaterThanOrEqual(0);
    expect(geometry.x + geometry.width).toBeLessThanOrEqual(width);
    expect(geometry.y + geometry.height).toBeLessThanOrEqual(844);
    expect(geometry.width / geometry.height).toBeCloseTo(geometry.ratio, 2);
    await page.screenshot({ path: `test-results/attachment-viewer-single-${width}.png`, animations: "disabled" });
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await expect(firstTrigger).toBeFocused();
    await expect(page.locator("textarea")).toHaveValue("Local upload check");

    await drop(page, "second.png", "/assets/images/workspace/neirohub-how-it-works-poster.png");
    await expect(tiles.last()).toHaveAttribute("data-status", "ready");
    const secondSource = await tiles.last().getByRole("img").getAttribute("src");
    await tiles.last().getByRole("button", { name: "Просмотр файла: second.png" }).click();
    await expect(previewImage).toHaveAttribute("src", secondSource!);
    await expect(dialog.getByTestId("attachment-preview-thumbnail")).toHaveCount(2);
    await dialog.getByRole("button", { name: "Предыдущий файл" }).click();
    await expect(previewImage).not.toHaveAttribute("src", secondSource!);
    await page.keyboard.press("ArrowRight");
    await expect(previewImage).toHaveAttribute("src", secondSource!);
    await dialog.getByTestId("attachment-preview-thumbnail").first().click();
    await expect.poll(() => previewImage.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBeGreaterThan(0);
    await page.screenshot({ path: `test-results/attachment-viewer-multiple-${width}.png`, animations: "disabled" });
    await dialog.getByRole("button", { name: "Закрыть предпросмотр" }).click();
    await expect(dialog).toBeHidden();
    await page.locator("form").getByRole("button", { name: "Отправить", exact: true }).click();
    const sent = page.locator('[data-ui="conversation-input-image"]');
    await expect(sent).toHaveCount(2);
    await expect.poll(() => sent.last().getByRole("img").evaluate((img: HTMLImageElement) => img.naturalWidth)).toBeGreaterThan(0);
    await sent.last().getByRole("button").click();
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole("complementary")).toHaveCount(0);
    await expect(previewImage).toHaveAttribute("src", (await sent.last().getByRole("img").getAttribute("src"))!);
    await expect(dialog.getByTestId("attachment-preview-thumbnail")).toHaveCount(2);
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await expect(sent.last().getByRole("button")).toBeFocused();
  });
}

test("uploads, fails, retries and cancels using only the installed local simulator", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  const outgoing: string[] = [];
  page.on("request", request => { if (/\/web\/v1\/(input-artifacts|image-reference-quote)/.test(request.url())) outgoing.push(request.url()); });
  await composer(page);
  await drop(page);
  const tile = page.locator('[data-ui="chat-attachment"]');
  await expect(tile.getByRole("progressbar")).toBeVisible();
  await expect(tile).toHaveAttribute("data-status", "ready");
  await expect(page.locator("form").getByRole("button", { name: "Отправить", exact: true })).toBeEnabled();
  await tile.hover(); await tile.getByRole("button", { name: "Удалить файл: local.png" }).click();
  await scenario(page, "Ошибка"); await drop(page);
  await expect(tile).toHaveAttribute("data-status", "failed");
  await scenario(page, "Успех");
  await tile.getByRole("button", { name: "Повторить загрузку: local.png" }).click();
  await expect(tile).toHaveAttribute("data-status", "ready");
  await tile.hover(); await tile.getByRole("button", { name: "Удалить файл: local.png" }).click();
  await scenario(page, "Медленно"); await drop(page);
  await expect.poll(async () => Number(await tile.getByRole("progressbar").getAttribute("aria-valuenow"))).toBeGreaterThan(0);
  await tile.hover(); await tile.getByRole("button", { name: "Удалить файл: local.png" }).click();
  await expect(tile).toHaveCount(0);
  await expect(page.locator("textarea")).toHaveValue("Local upload check");
  await page.reload();
  await expect(page.locator('[data-ui="local-development-tools"]')).toContainText("Медленно");
  expect(outgoing).toEqual([]);
  await page.locator('[data-ui="local-development-tools"] button').click();
  await expect(page.getByRole("dialog", { name: "Локальная проверка" })).toHaveCSS("opacity", "1");
  await page.screenshot({ path: "test-results/local-development-desktop.png", animations: "disabled" });
});

test("supports quote errors, duplicate detection and retry after switching to success", async ({ page }) => {
  await composer(page);
  await page.locator('[data-ui="local-development-tools"] button').click();
  await page.getByRole("toolbar", { name: "Расчёт стоимости" }).getByRole("button", { name: "Ошибка", exact: true }).click();
  await page.getByRole("button", { name: "Закрыть локальную проверку" }).click();
  await drop(page); await expect(page.locator('[data-ui="chat-attachment"]')).toHaveAttribute("data-status", "ready");
  await expect(page.getByRole("button", { name: "Повторить расчёт" })).toBeVisible();
  await drop(page, "renamed.png");
  await expect(page.getByRole("alertdialog")).toBeVisible();
  await page.getByRole("button", { name: "Понятно" }).click();
  await page.locator('[data-ui="local-development-tools"] button').click();
  await page.getByRole("toolbar", { name: "Расчёт стоимости" }).getByRole("button", { name: "Успех", exact: true }).click();
  await page.getByRole("button", { name: "Закрыть локальную проверку" }).click();
  await page.getByRole("button", { name: "Повторить расчёт" }).click();
  await expect(page.getByRole("button", { name: "Повторить расчёт" })).toHaveCount(0);
  await expect(page.locator('[data-ui="chat-attachment"]')).toHaveCount(1);
});

test("fits the English controls on a touch screen", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await composer(page, "en");
  await page.locator('[data-ui="local-development-tools"] button').click();
  const panel = page.getByRole("dialog", { name: "Local testing" });
  await expect(panel).toBeVisible();
  await panel.getByRole("toolbar", { name: "Photo upload" }).getByRole("button", { name: "Error", exact: true }).click();
  await expect(panel).toHaveCSS("opacity", "1");
  await page.screenshot({ path: "test-results/local-development-mobile.png", animations: "disabled" });
  const box = (await panel.boundingBox())!;
  expect(box.x).toBeGreaterThanOrEqual(0); expect(box.x + box.width).toBeLessThanOrEqual(390);
  expect(box.y + box.height).toBeLessThanOrEqual(844);
  await page.keyboard.press("Escape"); await expect(panel).toBeHidden();
  await drop(page); await expect(page.locator('[data-ui="chat-attachment"]')).toHaveAttribute("data-status", "failed");
});

test("keeps two attached photos visible after sending and receives a local reply", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await composer(page);
  const outgoing: string[] = [];
  page.on("request", request => { if (/\/web\/v1\/(conversations|input-artifacts)/.test(request.url())) outgoing.push(request.url()); });
  await drop(page);
  await drop(page, "second.png", "/assets/images/workspace/neirohub-how-it-works-poster.png");
  await expect(page.locator('[data-ui="chat-attachment"][data-status="ready"]')).toHaveCount(2);
  await page.locator("textarea").fill("Два локальных фото");
  await page.locator("form").getByRole("button", { name: "Отправить", exact: true }).click();
  const message = page.getByText("Два локальных фото", { exact: true }).locator("..");
  await expect(message.getByRole("img", { name: "Прикреплённое изображение" })).toHaveCount(2);
  await expect.poll(() => message.getByRole("img").evaluateAll(images => images.every(image => (image as HTMLImageElement).naturalWidth > 0)), { timeout: 4000 }).toBe(true);
  await expect(page.getByText("Не отправлено", { exact: true })).toHaveCount(0);
  await expect(page.getByText(/Тестовый ответ: сообщение и вложения получены/)).toBeVisible();
  await expect(message.getByRole("img")).toHaveCount(2);
  await expect.poll(() => message.getByRole("img").evaluateAll(images => images.every(image => (image as HTMLImageElement).naturalWidth > 0))).toBe(true);
  const imageBoxes = await message.getByRole("img").evaluateAll(images => images.map(image => ({ bottom: image.getBoundingClientRect().bottom, height: image.getBoundingClientRect().height })));
  const textBox = (await page.getByText("Два локальных фото", { exact: true }).boundingBox())!;
  expect(imageBoxes.every(box => box.height <= 160 && box.bottom <= textBox.y)).toBe(true);
  await expect(page.locator('[data-ui="chat-attachment"]')).toHaveCount(0);
  await page.screenshot({ path: "test-results/local-sent-photos.png", animations: "disabled" });
  await page.getByRole("link", { name: "Идеи для проекта", exact: true }).click();
  await expect(page.getByText("Два локальных фото", { exact: true })).toHaveCount(0);
  await page.getByRole("link", { name: "Журавль на облаке", exact: true }).click();
  await expect(message.getByRole("img")).toHaveCount(2);
  await expect.poll(() => message.getByRole("img").evaluateAll(images => images.every(image => (image as HTMLImageElement).naturalWidth > 0))).toBe(true);
  expect(outgoing).toEqual([]);
});

test("retains the photo after a failed send and retries without a duplicate", async ({ page }) => {
  await composer(page);
  await scenario(page, "Ошибка", "Отправка сообщения");
  await drop(page);
  await expect(page.locator('[data-ui="chat-attachment"]')).toHaveAttribute("data-status", "ready");
  await page.locator("textarea").fill("Повтор сообщения с фото");
  await page.locator("form").getByRole("button", { name: "Отправить", exact: true }).click();
  await expect(page.getByText("Не отправлено", { exact: true })).toBeVisible();
  const message = page.getByText("Повтор сообщения с фото", { exact: true }).locator("..");
  await expect.poll(() => message.getByRole("img", { name: "Прикреплённое изображение" }).evaluate(image => (image as HTMLImageElement).naturalWidth)).toBeGreaterThan(0);
  await scenario(page, "Успех", "Отправка сообщения");
  await message.getByRole("button", { name: "Повторить", exact: true }).click();
  await expect(page.getByText(/Тестовый ответ: сообщение и вложения получены/)).toBeVisible();
  await expect(page.getByText("Не отправлено", { exact: true })).toHaveCount(0);
  await expect(page.getByText("Повтор сообщения с фото", { exact: true })).toHaveCount(1);
  await expect.poll(() => message.getByRole("img", { name: "Прикреплённое изображение" }).evaluate(image => (image as HTMLImageElement).naturalWidth)).toBeGreaterThan(0);
});

test("creates a new local chat with a photo and a delayed reply", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await composer(page, "ru", true);
  await scenario(page, "Медленно", "Отправка сообщения");
  await drop(page);
  await expect(page.locator('[data-ui="chat-attachment"]')).toHaveAttribute("data-status", "ready");
  await page.locator("textarea").fill("Первое сообщение с фото");
  await page.locator("form").getByRole("button", { name: "Начать чат", exact: true }).click();
  await expect(page).toHaveURL(/\/ru\/app\/chat\/[a-f0-9-]+\?refresh=1/);
  const message = page.locator("li").filter({ has: page.locator("p").filter({ hasText: /^Первое сообщение с фото$/ }) });
  await expect(message.getByRole("img", { name: "Прикреплённое изображение" })).toHaveCount(1);
  await expect.poll(() => message.getByRole("img", { name: "Прикреплённое изображение" }).evaluate(image => (image as HTMLImageElement).naturalWidth)).toBeGreaterThan(0);
  await expect(page.getByText(/Тестовый ответ: сообщение и вложения получены/)).toHaveCount(0);
  await expect(page.getByText(/Тестовый ответ: сообщение и вложения получены/)).toBeVisible({ timeout: 12_000 });
  await expect(page.getByText("Не отправлено", { exact: true })).toHaveCount(0);
  await page.screenshot({ path: "test-results/local-new-chat-photo.png", animations: "disabled" });
});

for (const width of [1100, 390]) {
  test(`shows the floating typing control only above the chat bottom at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 820 });
    await page.emulateMedia({ reducedMotion: "reduce", colorScheme: "dark" });
    await composer(page);
    await scenario(page, "Медленно", "Отправка сообщения");
    await page.locator("form").getByRole("button", { name: "Отправить", exact: true }).click();
    const region = page.getByTestId("workspace-scroll-region");
    const button = page.getByRole("button", { name: "К последнему сообщению", exact: true });
    const typing = page.locator('[data-chat-pending="assistant"]').getByRole("status", { name: "NeiroHub печатает" });
    await expect(typing).toBeVisible();
    await expect.poll(() => region.evaluate(element => element.scrollHeight - element.scrollTop - element.clientHeight)).toBeLessThanOrEqual(4);
    await expect(button).toHaveCount(0);

    await region.evaluate(element => { element.scrollTop -= 250; });
    await expect(button.getByRole("status", { name: "NeiroHub печатает" })).toBeVisible();
    await button.click();
    await expect(button).toHaveCount(0);
    await expect(typing).toBeVisible();

    await region.evaluate(element => { element.scrollTop -= 250; });
    await expect(button.getByRole("status", { name: "NeiroHub печатает" })).toBeVisible();
    await expect(page.getByText(/Тестовый ответ: сообщение и вложения получены/)).toBeAttached({ timeout: 12_000 });
    await expect(button).toBeVisible();
    await expect(button.getByRole("status")).toHaveCount(0);
    await expect(button.locator("svg")).toHaveCount(1);
    await button.click();
    await expect(button).toHaveCount(0);
  });
}
