import { expect, test } from "@playwright/test";

test("opening a legal document keeps matching rounded navigation and reading panels", async ({ page }) => {
  await page.goto("/ru/legal");
  const reading = page.locator("[data-help-reading]");
  const header = page.locator("[data-legal-page] header");
  const sidebar = page.locator("[data-legal-page] header + div > aside");
  await expect(reading).toHaveCSS("background-color", "rgb(21, 27, 35)");
  await expect(sidebar).toHaveCSS("background-color", "rgb(21, 27, 35)");
  await expect(reading).toHaveCSS("border-radius", "12px");
  await expect(sidebar).toHaveCSS("border-radius", "12px");
  await expect(header).toHaveCSS("background-color", "rgb(13, 17, 23)");

  await reading.getByRole("link", { name: /Согласие на рекламные письма/ }).click();
  await expect(page).toHaveURL(/\/ru\/legal\/marketing-consent$/);
  await expect(reading).toHaveCSS("background-color", "rgb(21, 27, 35)");
  await expect(sidebar).toHaveCSS("background-color", "rgb(21, 27, 35)");
  await expect(reading).toHaveCSS("border-radius", "12px");
  await expect(sidebar).toHaveCSS("border-radius", "12px");
  await expect(header).toHaveCSS("background-color", "rgb(13, 17, 23)");
});
