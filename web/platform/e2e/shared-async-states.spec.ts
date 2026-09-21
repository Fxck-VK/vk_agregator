import { expect, test } from "@playwright/test";

for (const width of [1440, 390]) {
  test(`shared states are centered and recover at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 950 });
    await page.emulateMedia({ colorScheme: width === 390 ? "light" : "dark", reducedMotion: "reduce" });
    await page.goto("/ru/app/ui-states");
    await page.getByRole("button", { name: "Ошибка", exact: true }).click();
    const media = page.locator("#media");
    const button = media.getByRole("button", { name: "Повторить", exact: true });
    const area = await media.locator('[data-ui="media-state"]').boundingBox();
    const target = await button.boundingBox();
    expect(Math.abs(target!.x + target!.width / 2 - area!.x - area!.width / 2)).toBeLessThan(1);
    expect(Math.abs(target!.y + target!.height / 2 - area!.y - area!.height / 2)).toBeLessThan(1);
    expect(await button.evaluate(node => getComputedStyle(node).backgroundColor)).toBe("rgba(0, 0, 0, 0)");
    await button.click();
    await expect(media.locator('[data-ui="media-image"]')).toHaveAttribute("data-state", "ready");
    expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false);
    await page.getByRole("button", { name: "Загрузка", exact: true }).click();
    await expect(media.getByRole("progressbar")).toHaveCSS("animation-name", "none");
  });
}

test("failed image bytes can be retried and opened in the existing viewer", async ({ page }) => {
  let fail = true;
  await page.route("**/web/v1/image-artifacts/*", route => fail ? route.abort("failed") : route.continue());
  await page.goto("/ru/app/files?category=images");
  const card = page.getByRole("tabpanel").getByRole("article").first();
  await expect(card.getByRole("button", { name: "Повторить", exact: true })).toBeVisible();
  expect(await card.locator("button button").count()).toBe(0);
  fail = false;
  await card.getByRole("button", { name: "Повторить", exact: true }).click();
  await expect(card.locator('[data-ui="media-image"]')).toHaveAttribute("data-state", "ready");
  await card.getByRole("button", { name: /^Открыть файл:/ }).click();
  await expect(page.getByRole("dialog").locator('[data-ui="media-image"][data-fit="contain"]')).toHaveAttribute("data-state", "ready");
});

test("file metadata loading uses shared placeholders without the old technical text", async ({ page }) => {
  let release: () => void = () => undefined;
  const gate = new Promise<void>(resolve => { release = resolve; });
  await page.route("**/web/v1/image-jobs/*/result", async route => { await gate; await route.continue(); });
  try {
    await page.goto("/ru/app/files?category=images");
    const panel = page.getByRole("tabpanel");
    await expect(panel.locator('[data-ui="media-state"]').first()).toBeVisible();
    await expect(panel.getByText("Изображение будет показано при прокрутке.")).toHaveCount(0);
    await expect(panel.getByText("Готово", { exact: true })).toHaveCount(0);
  } finally { release(); }
});
