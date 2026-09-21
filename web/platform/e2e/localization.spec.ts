import { expect, test } from "@playwright/test";

test.skip(process.env.NEIROHUB_LOCAL_WORKSPACE_PREVIEW !== "1", "Requires the read-only local workspace fixture.");

test("locale cookie controls SSR and stays isolated between browser sessions", async ({ browser, baseURL }) => {
  const english = await browser.newContext({ baseURL });
  const russian = await browser.newContext({ baseURL });
  try {
    await english.addCookies([{ name: "neirohub-locale", value: "en", url: baseURL! }]);
    const [en, ru] = await Promise.all([english.request.get("/app/files"), russian.request.get("/app/files")]);
    expect(await en.text()).toMatch(/<html[^>]*lang="en"/);
    expect(await ru.text()).toMatch(/<html[^>]*lang="ru"/);
    expect(await en.text()).toMatch(/<h1[^>]*>My files<\/h1>/);
    expect(await ru.text()).toMatch(/<h1[^>]*>Мои файлы<\/h1>/);
    const page = await english.newPage();
    await page.goto("/app/files");
    await expect(page.getByRole("heading", { name: "My files" })).toBeVisible();
    await page.getByRole("button", { name: /^Open file:/ }).first().click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByRole("button", { name: "Animate", exact: true })).toBeVisible();
    await expect(dialog.getByText("Size", { exact: true })).toBeVisible();
    await expect(dialog.locator("time, dd").filter({ hasText: /Sep|Jan|Aug/ }).first()).toBeVisible();
  } finally { await english.close(); await russian.close(); }
});

test("switching language preserves the draft and selected model and survives reload", async ({ page, context, baseURL }) => {
  await context.addCookies([{ name: "neirohub-locale", value: "en", url: baseURL! }]);
  await page.goto("/app/chat/20000000-0000-4000-8000-000000000004");
  const input = page.locator("textarea");
  const header = page.locator('button[aria-controls="workspace-model-selector-dialog"]');
  await expect(header).not.toContainText("Loading");
  const model = await header.innerText();
  await input.fill("Мой черновик — do not translate");
  await page.locator("[data-sidebar-account-trigger]").click();
  await page.getByRole("button", { name: "Русский", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("lang", "ru");
  await expect(input).toHaveValue("Мой черновик — do not translate");
  await expect(header).toHaveText(model);
  if (!(await page.getByRole("button", { name: "English", exact: true }).isVisible())) await page.locator("[data-sidebar-account-trigger]").click();
  await page.getByRole("button", { name: "English", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  await expect(input).toHaveValue("Мой черновик — do not translate");
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  await expect(page.getByText("Журавль на облаке", { exact: true })).toBeVisible();
});

test("long labels and RTL keep narrow layouts and the selected-tab indicator usable", async ({ page, context, baseURL }) => {
  await context.addCookies([{ name: "neirohub-locale", value: "en", url: baseURL! }]);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/app/files");
  await expect(page.getByRole("tab", { name: "Uploaded" })).toBeAttached();
  for (const direction of ["ltr", "rtl"]) {
    await page.evaluate(dir => {
      document.documentElement.dir = dir;
      document.querySelectorAll('[role="tab"]').forEach((tab, index) => { tab.textContent = `A deliberately long translated category ${index}`; });
      window.dispatchEvent(new Event("resize"));
    }, direction);
    await page.getByRole("tab").last().click();
    await expect.poll(() => page.getByRole("tablist").evaluate(list => {
      const active = list.querySelector('[aria-selected="true"]')!.getBoundingClientRect();
      const indicator = list.querySelector('[data-testid="mode-switch-panel-indicator"]')!.getBoundingClientRect();
      return Math.max(Math.abs(active.left - indicator.left), Math.abs(active.right - indicator.right));
    })).toBeLessThan(3);
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390);
    await page.getByRole("tab").last().press("Home");
    await expect(page.getByRole("tab").first()).toHaveAttribute("aria-selected", "true");
  }
  await page.getByRole("button", { name: "Open menu", exact: true }).click();
  const panel = page.getByTestId("sidebar-panel");
  await expect(panel).toBeVisible();
  await expect.poll(async () => Math.abs((await panel.boundingBox())!.x + (await panel.boundingBox())!.width - 390)).toBeLessThan(2);
  await page.getByRole("button", { name: "Collapse sidebar", exact: true }).click();
  await expect(panel).toBeHidden();
});
