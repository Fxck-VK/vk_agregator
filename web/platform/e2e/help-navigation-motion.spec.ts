import {expect, test} from "@playwright/test";

test.use({viewport: {width: 1440, height: 900}, reducedMotion: "no-preference"});

test("document groups animate in both directions and retain their selected item", async ({page}) => {
  await page.goto("/ru/legal/privacy-policy");
  const group = page.locator("[data-help-navigation] details")
    .filter({has: page.locator('a[href="/ru/legal/marketing-consent"]')});

  for (const direction of ["close", "open"]) {
    const samples = await group.evaluate(async element => {
      const heights: number[] = [];
      const opacity: number[] = [];
      element.querySelector("summary")!.click();
      const started = performance.now();
      while (performance.now() - started < 320) {
        const style = getComputedStyle(element, "::details-content");
        heights.push(parseFloat(style.blockSize));
        opacity.push(Number(style.opacity));
        await new Promise(resolve => requestAnimationFrame(resolve));
      }
      return {heights, opacity};
    });
    const maximum = Math.max(...samples.heights);
    expect(maximum).toBeGreaterThan(0);
    expect(samples.heights.some(height => height > 0 && height < maximum)).toBe(true);
    expect(samples.opacity.some(value => value > 0 && value < 1)).toBe(true);
    expect(samples.heights.at(-1), direction).toBe(direction === "close" ? 0 : maximum);
  }
  await expect(group.locator('a[aria-current="page"]')).toHaveCSS("border-inline-start-color", "rgb(154, 124, 245)");
});

test("changing documents fades the navigation and preserves expanded groups", async ({page}) => {
  await page.addInitScript(() => {
    const start = document.startViewTransition.bind(document);
    document.startViewTransition = options => {
      const transition = start(options);
      document.documentElement.dataset.helpTransition = "started";
      void transition.finished.then(() => {
        document.documentElement.dataset.helpTransition = "finished";
      });
      return transition;
    };
  });
  await page.goto("/ru/legal/privacy-policy");
  const sidebar = page.locator("[data-help-navigation]");
  const otherGroup = sidebar.locator("details").filter({hasText: "Условия использования"});
  await otherGroup.locator("summary").click();
  await expect(otherGroup).toHaveAttribute("open");

  await sidebar.getByRole("link", {name: "Согласие на рекламные письма", exact: true}).click();
  await expect(page).toHaveURL(/\/legal\/marketing-consent$/);
  await expect(page.locator("html")).toHaveAttribute("data-help-transition", "finished");
  await expect(otherGroup).toHaveAttribute("open");
  await expect(sidebar.locator('a[aria-current="page"]')).toHaveCSS("font-weight", "600");
  await expect(sidebar.locator('a[aria-current="page"]')).toHaveCSS("border-inline-start-color", "rgb(154, 124, 245)");
});

test("reduced motion keeps navigation immediate and selectable", async ({page}) => {
  await page.emulateMedia({reducedMotion: "reduce"});
  await page.goto("/ru/legal/privacy-policy");
  const group = page.locator("[data-help-navigation] details")
    .filter({has: page.locator('a[href="/ru/legal/marketing-consent"]')});
  await expect(group).toHaveCSS("interpolate-size", "allow-keywords");
  expect(await group.evaluate(el => getComputedStyle(el, "::details-content").transitionDuration)).toBe("0s");
  await group.getByRole("link", {name: "Согласие на рекламные письма", exact: true}).click();
  await expect(page).toHaveURL(/\/legal\/marketing-consent$/);
  await expect(page.locator('[data-help-navigation] a[aria-current="page"]')).toHaveText("Согласие на рекламные письма");
});
