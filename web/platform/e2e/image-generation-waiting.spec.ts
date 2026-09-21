import { expect, test } from "@playwright/test";

for (const width of [1440, 390]) {
  test(`image waiting keeps count, ratio and centered indicators at ${width}px`, async ({ page }) => {
    const mutations: string[] = [];
    page.on("request", request => { if (request.method() === "POST") mutations.push(request.url()); });
    await page.setViewportSize({ width, height: 950 });
    await page.emulateMedia({ colorScheme: width === 390 ? "light" : "dark", reducedMotion: "reduce" });
    await page.goto("/ru/app/ui-states#image-generation");
    const batch = page.locator("#image-generation");
    const grid = batch.locator('[data-ui="image-generation-grid"]');
    await expect(batch.getByRole("progressbar")).toHaveCount(4);
    await expect(batch.getByRole("progressbar").first()).not.toHaveAttribute("aria-valuenow");
    await expect(batch.getByRole("progressbar").first()).toHaveCSS("animation-name", "none");
    for (const ratio of ["1:1", "9:16", "16:9"]) {
      if (ratio !== "1:1") {
        await batch.getByRole("button", { name: /Соотношение сторон/ }).click();
        await page.getByLabel(ratio, { exact: true }).click();
      }
      const [a, b] = ratio.split(":").map(Number);
      await expect(grid).toHaveCSS("--generation-aspect-ratio", String(a / b));
      await expect.poll(async () => {
        const box = await grid.locator(":scope > div").first().boundingBox();
        return box!.width / box!.height;
      }).toBeCloseTo(a / b, 2);
      const frames = await grid.locator(":scope > div").evaluateAll(elements => elements.map(element => {
        const bounds = element.getBoundingClientRect();
        const indicator = element.querySelector('[role="progressbar"]')!.getBoundingClientRect();
        return { x: bounds.x, y: bounds.y, width: bounds.width, height: bounds.height,
          centerX: indicator.x + indicator.width / 2 - bounds.x - bounds.width / 2,
          centerY: indicator.y + indicator.height / 2 - bounds.y - bounds.height / 2 };
      }));
      for (const frame of frames) {
        expect(frame.width / frame.height).toBeCloseTo(a / b, 2);
        expect(Math.abs(frame.centerX)).toBeLessThan(1);
        expect(Math.abs(frame.centerY)).toBeLessThan(1);
      }
      expect(frames[0].y).toBe(frames[1].y);
      expect(frames[2].y).toBeGreaterThan(frames[0].y);
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    const pendingSize = await grid.boundingBox();
    await page.getByRole("button", { name: "Успех", exact: true }).click();
    await expect(batch.locator('[data-ui="media-image"][data-state="ready"]')).toHaveCount(4);
    const readySize = await grid.boundingBox();
    expect(readySize!.width).toBeCloseTo(pendingSize!.width, 0);
    expect(readySize!.height).toBeCloseTo(pendingSize!.height, 0);
    await page.getByRole("button", { name: "Ошибка", exact: true }).click();
    await expect(batch.getByRole("alert")).toBeVisible();
    await batch.getByRole("button", { name: "Повторить", exact: true }).click();
    await expect(batch.getByRole("progressbar")).toHaveCount(4);
    for (let index = 0; index < 3; index++) await batch.getByRole("button", { name: "Уменьшить количество" }).click();
    await expect(batch.getByRole("progressbar")).toHaveCount(1);
    expect(mutations).toEqual([]);
  });
}

test("image waiting labels follow the English locale", async ({ page }) => {
  await page.goto("/en/app/ui-states#image-generation");
  const batch = page.locator("#image-generation");
  await expect(batch.getByRole("heading", { name: "Waiting for multiple images" })).toBeVisible();
  await expect(batch.getByRole("progressbar", { name: "Generating image 4 of 4" })).toBeVisible();
});
