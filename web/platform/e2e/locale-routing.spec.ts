import { expect, test } from "@playwright/test";

test.skip(process.env.NEIROHUB_LOCAL_WORKSPACE_PREVIEW !== "1", "Requires the read-only local workspace fixture.");

test("URL language wins over cookies on every existing page; service routes and missing pages keep their boundaries", async ({ context, baseURL }) => {
  const request = context.request;
  await context.addCookies([{ name: "neirohub-locale", value: "ru", url: baseURL! }]);
  for (const locale of ["en", "ru"]) {
    for (const path of ["", "/login", "/app", "/app/models", "/app/files", "/app/profile", "/app/inspiration", "/app/image", "/app/chats", "/app/payment-return"]) {
      const response = await request.get(`/${locale}${path}`);
      expect(response.status(), `${locale}${path}`).toBe(200);
      const html = await response.text();
      expect(html).toMatch(new RegExp(`<html[^>]*lang="${locale}"`));
      if (path) expect(html).toMatch(/name="robots" content="noindex, nofollow"/);
    }
  }
  for (const path of ["/fr/app", "/ru/en/app", "/en/web/v1/models", "/en/health", "/en/not-a-page"]) {
    expect((await request.get(path)).status(), path).toBe(404);
  }
  expect((await request.get("/health")).status()).toBe(200);
  const api = await request.get("/web/v1/models");
  expect(api.headers()["content-type"]).toContain("application/json");
  const prefetch = await request.get("/en/app/files?_rsc=test", { headers: { rsc: "1", "next-router-prefetch": "1" } });
  expect(prefetch.headers()["set-cookie"]).toBeUndefined();
});

test("public metadata and public shell switch together; sitemap contains only localized public pages", async ({ page, request }) => {
  await page.goto("/ru");
  const errors: string[] = [];
  page.on("pageerror", error => errors.push(error.message));
  await page.getByRole("button", { name: "English", exact: true }).click();
  await expect(page).toHaveURL(/\/en$/);
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  await expect(page.locator("header a[href='/en/app']")).toHaveText("Open platform");
  await expect(page.locator("head link[rel=canonical]")).toHaveAttribute("href", "http://localhost:7158/en");
  await expect(page.locator('head link[hreflang="ru"]')).toHaveAttribute("href", "http://localhost:7158/ru");
  await expect(page.locator('head link[hreflang="en"]')).toHaveAttribute("href", "http://localhost:7158/en");
  await page.goBack();
  await expect(page.locator("html")).toHaveAttribute("lang", "ru");
  await expect(page.locator("header a[href='/ru/app']")).toHaveText("Открыть платформу");
  expect(errors).toEqual([]);
  const sitemap = await (await request.get("/sitemap.xml")).text();
  expect(sitemap.match(/<loc>/g)).toHaveLength(2);
  expect(sitemap).toContain("http://localhost:7158/en");
  expect(sitemap).not.toMatch(/\/app|\/login/);
});

test("switching a real chat preserves uploaded attachment, draft, model and URL suffix through history navigation", async ({ page, context, baseURL }) => {
  await context.addCookies([{ name: "nh_csrf", value: "local-test-only", url: baseURL! }]);
  let uploads = 0;
  await page.route("**/web/v1/input-artifacts?*", async route => {
    uploads++;
    await route.fulfill({ json: { artifact_id: "70000000-0000-4000-8000-000000000001", mime_type: "image/png", size_bytes: 68, width: 1, height: 1 } });
  });
  await page.route("**/web/v1/image-reference-quote?*", route => route.fulfill({ json: { credits: 20 } }));
  const path = "/app/chat/20000000-0000-4000-8000-000000000004?model=gpt-image-2&test=a%2Bb#draft";
  await page.goto(`/ru${path}`);
  const header = page.locator('button[aria-controls="workspace-model-selector-dialog"]');
  await expect(header).not.toContainText("Загрузка");
  await page.locator('[data-variant="composer"] > button').click();
  const selector = page.getByRole("dialog");
  await selector.getByRole("searchbox").fill("Seedream 5.0 Pro");
  await selector.getByRole("button", { name: /Seedream 5.0 Pro/ }).first().click();
  await expect(header).toContainText("Seedream 5.0 Pro");
  const model = await header.innerText();
  const input = page.locator("textarea");
  await input.fill("Сохранить черновик / keep this draft");
  await page.evaluate(() => {
    const transfer = new DataTransfer();
    const bytes = Uint8Array.from(atob("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aWQAAAABJRU5ErkJggg=="), character => character.charCodeAt(0));
    transfer.items.add(new File([bytes], "locale-test.png", { type: "image/png" }));
    document.querySelector('[data-ui="workspace-file-drop"]')!.dispatchEvent(new DragEvent("drop", { bubbles: true, dataTransfer: transfer }));
  });
  await expect.poll(() => uploads).toBe(1);
  const image = page.locator('img[src^="blob:"]');
  await expect(image).toBeVisible();
  const previewURL = await image.getAttribute("src");
  await page.locator("[data-sidebar-account-trigger]").click();
  await page.getByRole("button", { name: "English", exact: true }).click();
  await expect(page).toHaveURL(new URL(`/en${path}`, baseURL).href);
  await expect(input).toHaveValue("Сохранить черновик / keep this draft");
  await expect(header).toHaveText(model);
  await expect(image).toHaveAttribute("src", previewURL!);
  await page.goBack();
  await expect(page.locator("html")).toHaveAttribute("lang", "ru");
  await expect(input).toHaveValue("Сохранить черновик / keep this draft");
  await expect(image).toHaveAttribute("src", previewURL!);
  await page.goForward();
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  await expect(image).toHaveAttribute("src", previewURL!);
  expect(uploads).toBe(1);
});

test("legacy payment and login URLs keep their language and navigation identifiers", async ({ page, context, baseURL }) => {
  await context.addCookies([{ name: "neirohub-locale", value: "en", url: baseURL! }]);
  const paymentPath = "/app/payment-return?payment_id=90000000-0000-4000-8000-000000000001";
  const payment = await context.request.get(paymentPath, { maxRedirects: 0 });
  expect(payment.status()).toBe(307);
  expect(new URL(payment.headers().location, baseURL).href).toBe(`${baseURL}/en${paymentPath}`);
  await page.goto("/en/app/chats?model=gpt-image-2");
  await page.goto("/login");
  await expect(page).toHaveURL(/\/en\/login$/);
  await page.route("**/web/v1/auth/password/login", route => route.fulfill({ json: {} }));
  await page.locator('input[name="email"]').fill("local-test@example.invalid");
  await page.locator('input[name="password"]').fill("local-test-only");
  await page.locator('button[type="submit"]').click();
  await expect(page).toHaveURL(/\/en\/app\/chats\?model=gpt-image-2$/);
});
