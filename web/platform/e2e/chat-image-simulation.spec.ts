import { expect, test } from "@playwright/test";

test.skip(process.env.NEIROHUB_LOCAL_WORKSPACE_PREVIEW !== "1", "Requires local development preview.");

for (const count of [1, 2, 3, 4, 5]) {
  test(`chat waits for ${count} images and opens the simulated results`, async ({ page }) => {
    test.setTimeout(60000);
    const width = count === 3 ? 390 : 1440;
    await page.setViewportSize({ width, height: 950 });
    await page.emulateMedia({ colorScheme: count === 3 ? "light" : "dark" });
    await page.addInitScript(() => sessionStorage.setItem("neirohub.local-scenarios.v1", JSON.stringify({ enabled: true, upload: "success", quote: "success", message: "slow" })));
    const writes: string[] = [];
    const errors: string[] = [];
    page.on("request", request => { if (request.method() === "POST") writes.push(request.url()); });
    page.on("pageerror", error => errors.push(error.message));
    await page.goto(count === 2 ? "/ru/app/chats?model=seedream_5_0_lite" : "/ru/app/chat/20000000-0000-4000-8000-000000000004");
    await expect(page.locator('[data-ui="local-development-tools"]')).toBeVisible();
    if (count !== 2) {
      await page.locator('[data-variant="composer"] > button').click();
      await page.getByRole("dialog").getByRole("searchbox").fill("Seedream 5.0 Lite");
      await page.getByRole("dialog").getByRole("button", { name: /Seedream 5.0 Lite/ }).first().click();
    }
    const form = page.locator("form");
    for (let i = 1; i < count; i++) await form.getByRole("button", { name: "Увеличить количество" }).click();
    await form.getByRole("button", { name: /Соотношение сторон/ }).click();
    await page.getByLabel("9:16", { exact: true }).click();
    await page.locator("textarea").fill(`Локальная проверка ${count} фото`);
    const startedAt = Date.now();
    await form.getByRole("button", { name: count === 2 ? "Начать чат" : "Отправить", exact: true }).click();
    const grid = page.locator('[data-ui="image-generation-grid"]').last();
    await expect(grid).toHaveAttribute("data-count", String(count));
    await expect(grid.getByRole("progressbar")).toHaveCount(count);
    await expect(grid).toHaveCSS("--generation-aspect-ratio", String(9 / 16));
    await expect(grid.getByRole("progressbar").last()).toBeInViewport();
    await page.screenshot({ path: `test-results/chat-images-${count}-waiting.png`, animations: "disabled" });
    await expect(grid.locator('[data-ui="media-image"][data-state="ready"]')).toHaveCount(count, { timeout: 25000 });
    expect(Date.now() - startedAt).toBeGreaterThanOrEqual(8000);
    await expect(grid.getByRole("progressbar")).toHaveCount(0);
    const images = grid.getByRole("img");
    const sources = await images.evaluateAll(elements => elements.map(element => {
      const image = element as HTMLImageElement;
      return { src: image.src, ratio: image.naturalWidth / image.naturalHeight };
    }));
    expect(new Set(sources.map(image => image.src)).size).toBe(count);
    expect(sources.every(image => image.ratio === 9 / 16)).toBe(true);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await grid.getByRole("button", { name: /Открыть файл/ }).last().click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog.locator('img[src*="f1000000-"]').first()).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    expect(writes).toEqual([]);
    expect(errors).toEqual([]);
  });
}
