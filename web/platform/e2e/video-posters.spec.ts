import { expect, test } from "@playwright/test";

test.skip(process.env.NEIROHUB_LOCAL_WORKSPACE_PREVIEW !== "1", "Local preview only.");

test("video cards show small covers while MP4 is delayed or fails; offscreen media stays deferred", async ({ page }) => {
  let release!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  let allowVideo = false;
  const requests: string[] = [];
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.route("**/assets/videos/inspiration/*.mp4", async route => {
    requests.push(route.request().url());
    if (allowVideo) return route.continue();
    await gate;
    await route.fulfill({ status: 503, body: "Delayed video unavailable" });
  });
  await page.emulateMedia({ colorScheme: "dark" });
  await page.goto("/ru/app/inspiration");
  const card = page.getByRole("button", { name: "Открыть пример «Видеопример 1»", exact: true });
  const video = card.locator("video");
  await expect(video).not.toHaveAttribute("src");
  expect(requests).toHaveLength(0);
  await card.scrollIntoViewIfNeeded();
  const poster = card.locator('[data-ui="video-poster"]');
  await expect(poster).toBeVisible();
  await expect.poll(() => poster.evaluate(image => (image as HTMLImageElement).naturalWidth)).toBeGreaterThan(0);
  await expect.poll(() => requests.length).toBeGreaterThan(0);
  await expect(card.getByRole("progressbar")).toHaveCount(0);
  const before = await card.boundingBox();
  await page.screenshot({ path: ".tmp/video-posters-delayed.png" });
  release();
  await expect(card.locator('[data-ui="media-video"]')).toHaveAttribute("data-state", "error");
  await expect(poster).toBeVisible();
  allowVideo = true;
  await card.click();
  const dialog = page.getByRole("dialog");
  const preview = dialog.getByTestId("inspiration-media-surface");
  await expect.poll(() => preview.locator("video").evaluate(video => (video as HTMLVideoElement).readyState)).toBeGreaterThanOrEqual(2);
  await expect(preview.locator('[data-ui="video-poster"]')).toBeHidden();
  await expect(dialog.getByTestId("inspiration-thumbnail").locator("video")).toHaveCount(0);
  expect((await card.boundingBox())!.height).toBeCloseTo(before!.height, 0);
  expect(errors).toEqual([]);
});

test("a loaded card keeps hover playback and stable proportions", async ({ page }) => {
  await page.goto("/ru/app/inspiration");
  const card = page.getByRole("button", { name: "Открыть пример «Видеопример 2»", exact: true });
  await card.scrollIntoViewIfNeeded();
  const video = card.locator("video");
  await expect.poll(() => video.evaluate(video => (video as HTMLVideoElement).readyState)).toBeGreaterThanOrEqual(2);
  await card.hover();
  await expect.poll(() => video.evaluate(video => (video as HTMLVideoElement).currentTime)).toBeGreaterThan(0);
  const bounds = await card.boundingBox();
  expect(bounds!.width / bounds!.height).toBeCloseTo(1244 / 1664, 2);
  await page.mouse.move(0, 0);
  await expect.poll(() => video.evaluate(video => (video as HTMLVideoElement).paused)).toBe(true);
});
