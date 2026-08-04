import { expect, test } from "@playwright/test";

test("public homepage renders its SEO and content shell without a session", async ({ page }) => {
  const errors: string[] = [];
  page.on("console", (message) => { if (message.type() === "error") errors.push(message.text()); });

  await page.goto("/");

  await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
  await expect(page.getByRole("heading", { name: "Популярные нейросети" })).toBeVisible();
  await expect(page.locator('link[rel="canonical"]')).toHaveAttribute("href", "https://neiirohub.ru");
  await expect(page.locator('script[type="application/ld+json"]')).toHaveCount(1);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);

  const images = page.locator("main img");
  for (let index = 0; index < await images.count(); index += 1) {
    const image = images.nth(index);
    await image.scrollIntoViewIfNeeded();
    await expect.poll(() => image.evaluate((element) => element.naturalWidth)).toBeGreaterThan(0);
  }

  expect(errors).toEqual([]);
});

test("public controls work without reloading the homepage", async ({ page }) => {
  await page.goto("/");

  await page.getByRole("button", { name: /GPT Image/ }).click();
  await expect(page.getByLabel("Описание изображения")).toBeVisible();
  await expect(page.getByText("Стоимость: 16 ★")).toBeVisible();

  await page.getByRole("button", { name: "Показать ещё" }).click();
  await expect(page.getByTestId("public-model-card")).toHaveCount(10);

  await page.getByRole("button", { name: "Следующая новость" }).click();
  await expect(page.getByRole("heading", { name: "Генерация изображений с прозрачной ценой" })).toBeVisible();

  const secondQuestion = page.locator("summary").filter({ hasText: "Нужен ли VPN?" });
  await secondQuestion.click();
  await expect(secondQuestion.locator("..")).toHaveAttribute("open", "");
});

test("public theme controls persist light and dark preferences", async ({ page }) => {
  await page.goto("/");

  await page.getByRole("button", { name: "Тёмная тема" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("neirohub.theme"))).toBe("dark");

  await page.getByRole("button", { name: "Светлая тема" }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("neirohub.theme"))).toBe("light");
});

test("mobile drawer closes after navigation selection", async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 });
  await page.goto("/");
  await page.getByRole("button", { name: "Открыть меню" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.getByRole("dialog").getByRole("link", { name: "Мои файлы" }).click();
  await expect(page).toHaveURL(/\/login\?next=\/app\/files/);
});

test("guest prompt is saved before login redirect", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("Задайте вопрос NeiroHub").fill("Составь план запуска продукта");
  await page.getByLabel("Задайте вопрос NeiroHub").press("Enter");
  await expect(page).toHaveURL(/\/login\?next=\/app\/chats/);
  expect(await page.evaluate(() => sessionStorage.getItem("neirohub.guest-draft"))).toContain("Составь план запуска продукта");
  expect(await page.evaluate(() => localStorage.getItem("neirohub.guest-draft"))).toBeNull();
});
