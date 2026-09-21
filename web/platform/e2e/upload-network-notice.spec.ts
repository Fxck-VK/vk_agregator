import { expect, test, type Page } from "@playwright/test";

test.skip(process.env.NEIROHUB_LOCAL_WORKSPACE_PREVIEW !== "1", "Requires local workspace preview.");

const photo = "public/assets/images/inspiration/paper-crane-cloud.png";
const secondPhoto = "public/assets/images/models/default-model-87465de8.png";

async function openComposer(page: Page, locale = "ru") {
  await page.goto(`/${locale}/app/chat/20000000-0000-4000-8000-000000000004`);
  await page.locator('[data-variant="composer"] > button').click();
  const picker = page.getByRole("dialog");
  await picker.getByRole("searchbox").fill("Seedream 5.0 Pro");
  await picker.getByRole("button", { name: /Seedream 5.0 Pro/ }).first().click();
  await page.locator("textarea").fill("Network notice check");
}

async function expectAligned(page: Page) {
  const notice = page.getByTestId("chat-upload-network-notice");
  const input = page.locator('[data-ui="input-surface"]').filter({ has: page.locator("textarea") });
  await expect.poll(async () => {
    const banner = (await notice.boundingBox())!;
    const anchor = (await input.boundingBox())!;
    const workspace = (await page.locator('[data-ui="workspace-file-drop"]').boundingBox())!;
    return Math.max(Math.abs(banner.x - anchor.x), Math.abs(banner.width - anchor.width), Math.abs(banner.y - workspace.y - 16));
  }).toBeLessThan(1);
}

test("real offline uploads share one notice aligned with the input through scrolling and resizing", async ({ page, context, baseURL }) => {
  await context.addCookies([{ name: "nh_csrf", value: "local-test-only", url: baseURL! }]);
  await page.addInitScript(() => sessionStorage.setItem("neirohub.local-scenarios.v1", JSON.stringify({ enabled: false, upload: "success", quote: "success" })));
  await page.setViewportSize({ width: 1280, height: 844 });
  await page.emulateMedia({ colorScheme: "dark" });
  await openComposer(page);
  await context.setOffline(true);
  await page.locator('input[type="file"]').first().setInputFiles([photo, secondPhoto]);
  const notice = page.getByTestId("chat-upload-network-notice");
  const tiles = page.locator('[data-ui="chat-attachment"]');
  await expect(notice).toHaveCount(1);
  await expect(page.locator('[data-ui="chat-attachment"][data-status="failed"]')).toHaveCount(2);
  await expect(page.locator("textarea")).toBeFocused();
  await expectAligned(page);
  await page.getByRole("button", { name: "Свернуть боковую панель", exact: true }).click();
  await expect(page.getByTestId("app-shell")).toHaveAttribute("data-desktop-sidebar-collapsed", "true");
  await expectAligned(page);
  await page.getByTestId("workspace-scroll-region").evaluate(element => element.scrollTo(0, element.scrollHeight));
  await expectAligned(page);
  for (const width of [1000, 390]) {
    await page.setViewportSize({ width, height: 844 });
    await expectAligned(page);
  }
  await page.screenshot({ path: "test-results/network-notice-mobile.png", animations: "disabled" });
  await notice.getByRole("button", { name: "Закрыть уведомление" }).click();
  await expect(notice).toBeHidden();
  await expect(page.locator("textarea")).toBeFocused();
  await tiles.first().getByRole("button", { name: /Повторить загрузку/ }).click();
  await expect(notice).toBeVisible();
  await tiles.first().getByRole("button", { name: /Просмотр файла:/ }).click({ position: { x: 10, y: 10 } });
  const preview = page.getByRole("dialog", { name: "Просмотр файла", exact: true });
  await expect(preview).toBeVisible();
  expect(await preview.getByRole("img").evaluate(image => (image as HTMLImageElement).naturalWidth)).toBeGreaterThan(0);
  await page.keyboard.press("Escape");
  await expect(preview).toBeHidden();
  await context.setOffline(false);
  let uploads = 0;
  await page.route("**/web/v1/input-artifacts?*", route => route.fulfill({ json: {
    artifact_id: `70000000-0000-4000-8000-${String(++uploads).padStart(12, "0")}`, mime_type: "image/png", size_bytes: 1024, width: 1024, height: 1536,
  } }));
  await page.route("**/web/v1/image-reference-quote?*", route => route.fulfill({ json: { credits: 20 } }));
  for (const tile of await tiles.all()) await tile.getByRole("button", { name: /Повторить загрузку/ }).click();
  await expect(notice).toBeHidden();
  await expect(page.locator('[data-ui="chat-attachment"][data-status="ready"]')).toHaveCount(2);
  await expect(page.locator("textarea")).toHaveValue("Network notice check");
});

test("local offline scenario is selectable and successful retry clears the translated notice", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await openComposer(page, "en");
  await page.locator('[data-ui="local-development-tools"] button').click();
  const panel = page.getByRole("dialog", { name: "Local testing", exact: true });
  const upload = panel.getByRole("toolbar", { name: "Photo upload", exact: true });
  await upload.getByRole("button", { name: "Offline", exact: true }).click();
  await page.keyboard.press("Escape");
  await page.locator('input[type="file"]').first().setInputFiles(photo);
  const notice = page.getByTestId("chat-upload-network-notice");
  await expect(notice).toContainText("Check your connection and try again.");
  await expectAligned(page);
  await page.locator('[data-ui="local-development-tools"] button').click();
  await upload.getByRole("button", { name: "Success", exact: true }).click();
  await page.keyboard.press("Escape");
  await page.getByRole("button", { name: /Retry upload:/ }).click();
  await expect(page.locator('[data-ui="chat-attachment"]')).toHaveAttribute("data-status", "ready");
  await expect(notice).toBeHidden();
});
