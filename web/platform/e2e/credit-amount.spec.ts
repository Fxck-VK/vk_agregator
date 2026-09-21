import { expect, test } from "@playwright/test";

test.skip(process.env.NEIROHUB_LOCAL_WORKSPACE_PREVIEW !== "1", "Requires local preview.");

for (const [locale, width] of [["ru", 1280], ["en", 390]] as const) {
  test(`uses the shared currency icon in text pricing, packages and payment success (${locale})`, async ({ page }) => {
    await page.setViewportSize({ width, height: 844 });
    await page.emulateMedia({ colorScheme: "dark" });
    await page.goto(`/${locale}/app/chats?model=chatgpt`);
    const note = page.locator("form p").filter({ hasText: locale === "ru" ? "за ответ" : "per response" });
    await expect(note.getByTestId("credit-star-icon")).toBeVisible();
    await expect(note).toContainText("0");
    await expect(note).not.toContainText(/токен|tokens/);

    await page.goto(`/${locale}/app/chat/20000000-0000-4000-8000-000000000004`);
    await page.locator('[data-variant="composer"] > button').click();
    const selector = page.getByRole("dialog");
    await selector.getByRole("searchbox").fill("Claude Opus 4.8");
    await selector.getByRole("button", { name: /^Claude Opus 4\.8/ }).first().click();
    await expect(note.getByTestId("credit-star-icon")).toBeVisible();
    await expect(note).not.toContainText(/\d\s+токенов за ответ|\d\s+tokens per response/);
    await expect(note).toContainText(locale === "ru" ? "токенов ответа" : "output tokens");
    await page.screenshot({ path: `test-results/credit-chat-${locale}.png`, animations: "disabled" });

    await page.getByTestId("workspace-balance").click();
    const dialog = page.getByRole("dialog", { name: locale === "ru" ? "Пополнить баланс токенов" : "Top up tokens", exact: true });
    await expect(dialog.getByRole("radio")).toHaveCount(5);
    await expect(dialog.getByTestId("credit-star-icon")).toHaveCount(5);
    await expect(dialog.getByText(/^(ТОКЕНОВ|TOKENS)$/)).toHaveCount(0);
    const box = (await dialog.boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(width);
    await page.screenshot({ path: `test-results/credit-packages-${locale}.png`, animations: "disabled" });
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();

    const paymentID = "12345678-1234-4000-8000-123456789012";
    await page.route(`**/web/v1/payments/${paymentID}`, route => route.fulfill({ json: {
      id: paymentID, status: "succeeded", amount: 40000, currency: "RUB", credits: 800,
    } }));
    await page.evaluate(id => sessionStorage.setItem("neirohub:pending-payment", id), paymentID);
    await page.getByTestId("workspace-balance").click();
    await expect(dialog.getByRole("status").getByTestId("credit-star-icon")).toBeVisible();
    await expect(dialog.getByRole("status")).toContainText("800");
    await expect(dialog.getByRole("status")).not.toContainText(/токен|tokens/);
    await page.screenshot({ path: `test-results/credit-payment-${locale}.png`, animations: "disabled" });
  });
}
