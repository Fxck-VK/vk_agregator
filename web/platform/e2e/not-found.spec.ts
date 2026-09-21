import { expect, test } from "@playwright/test";

test("404 navigation and browser back do not produce performance timing errors", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  page.on("console", message => {
    if (message.type() === "error" && /negative time stamp/i.test(message.text())) errors.push(message.text());
  });

  for (const locale of ["en", "ru", "en", "ru"]) {
    const response = await page.goto(`/${locale}/app/missing-timing-${Date.now()}`);
    expect(response?.status()).toBe(404);
    await expect(page.locator("#not-found-title")).toBeVisible();
    await page.getByRole("link", { name: locale === "ru" ? "На главную" : "Back to home", exact: true }).click();
    await expect(page).toHaveURL(new RegExp(`/${locale}/app$`));
    await page.goBack();
    await expect(page.locator("#not-found-title")).toBeVisible();
    expect(errors).toEqual([]);
  }
});

for (const locale of ["ru", "en"]) {
  test(`missing pages keep the ${locale} language, 404 status and a working home link`, async ({ page }) => {
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    page.on("console", message => {
      if (message.type() === "error" && /script tag|hydration/i.test(message.text())) errors.push(message.text());
    });
    const response = await page.goto(`/${locale}/app/3333`);
    expect(response?.status()).toBe(404);
    await expect(page.locator("html")).toHaveAttribute("lang", locale);
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(
      locale === "ru" ? "Страница не найдена" : "Page not found",
    );
    const robots = await page.locator('meta[name="robots"]').evaluateAll(elements => elements.map(element => element.getAttribute("content")));
    expect(robots.length).toBeGreaterThan(0);
    expect(robots.every(content => content?.includes("noindex"))).toBe(true);
    await expect(page.getByTestId("app-shell")).toHaveCount(1);
    await expect(page.getByTestId("workspace-header")).toBeVisible();
    await expect(page.getByRole("navigation", { name: locale === "ru" ? "Основная навигация" : "Main navigation" })).toBeVisible();
    const character = page.locator('main img');
    await expect(character).toBeVisible();
    await expect.poll(() => character.evaluate((element: HTMLImageElement) => element.complete && element.naturalWidth > 0)).toBe(true);
    const home = page.getByRole("link", { name: locale === "ru" ? "На главную" : "Back to home" });
    await expect(home).toHaveAttribute("href", `/${locale}/app`);
    await home.click();
    await expect(page).toHaveURL(new RegExp(`/${locale}/app$`));
    await expect(page.getByRole("heading", { name: /Страница не найдена|Page not found/ })).toHaveCount(0);
    expect(errors).toEqual([]);
  });
}

test("404 fits a narrow screen, follows the light theme and respects reduced motion", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
  await page.goto("/ru/not-a-page");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Страница не найдена");
  const main = page.getByRole("main");
  await expect(main).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  expect(await page.getByRole("region", { name: "Страница не найдена" }).evaluate(element => getComputedStyle(element).backgroundColor)).toBe("rgb(250, 248, 244)");
  expect(await page.locator("main img").evaluate(element => getComputedStyle(element).animationName)).toBe("none");
  const home = page.getByRole("link", { name: "На главную" });
  await expect(home).toBeInViewport();
  await home.focus();
  await expect(home).toBeFocused();
  await home.hover();
  expect(await home.evaluate(element => getComputedStyle(element).backgroundColor)).toBe("rgba(0, 0, 0, 0)");
  await page.getByRole("button", { name: "Открыть меню", exact: true }).click();
  const menu = page.getByRole("navigation", { name: "Основная навигация" });
  await expect(menu).toBeVisible();
  await menu.getByRole("link", { name: "Все нейросети", exact: true }).click();
  await expect(page).toHaveURL(/\/ru\/app\/models$/);
});

test("404 preserves an explicitly selected theme when it differs from the system", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "light" });
  await page.addInitScript(() => localStorage.setItem("neirohub.theme", "dark"));
  const homeResponse = await page.request.get("/ru");
  expect(await homeResponse.text()).toMatch(/<script[^>]*nonce="[^"]+"[^>]*>[\s\S]*?neirohub\.theme/);
  await page.goto("/ru/another-missing-page");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.getByRole("link", { name: "На главную" }).click();
  await expect(page).toHaveURL(/\/ru\/app$/);
  await page.goBack();
  await expect(page.getByRole("heading", { name: "Страница не найдена" })).toBeVisible();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
});

test("public pages and missing static assets do not receive account data from the 404 shell", async ({ request }) => {
  for (const [path, status] of [["/ru", 200], ["/assets/missing-404-test.png", 404], ["/fr/app", 404], ["/en/web/v1/models", 404]] as const) {
    const response = await request.get(path);
    expect(response.status(), path).toBe(status);
    expect(await response.text(), path).not.toContain("preview@neirohub.local");
  }
});

test("a missing page outside /app keeps the working sidebar and browser back returns to 404", async ({ page }) => {
  const response = await page.goto("/ru/not-a-page/deeper");
  expect(response?.status()).toBe(404);
  await expect(page.getByTestId("app-shell")).toHaveCount(1);
  const content = page.getByRole("heading", { name: "Страница не найдена" });
  const sidebar = page.getByRole("navigation", { name: "Основная навигация" });
  await expect(content).toBeVisible();
  const [sidebarBox, contentBox] = await Promise.all([sidebar.boundingBox(), content.boundingBox()]);
  expect(contentBox!.x).toBeGreaterThan(sidebarBox!.x + sidebarBox!.width);
  await sidebar.getByRole("link", { name: "Мои файлы", exact: true }).click();
  await expect(page).toHaveURL(/\/ru\/app\/files$/);
  await expect(content).toHaveCount(0);
  await page.goBack();
  await expect(content).toBeVisible();
  await expect(page.getByTestId("app-shell")).toHaveCount(1);
  await page.getByRole("button", { name: "Свернуть боковую панель", exact: true }).click();
  await expect(page.getByTestId("app-shell")).toHaveAttribute("data-desktop-sidebar-collapsed", "true");
});
