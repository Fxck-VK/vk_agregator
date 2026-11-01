import { describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { PromotionEditor, PromotionList, PromotionsScreen, promotionMutationError } from "./PromotionsScreen";
import { AdminApiError } from "../api/adminClient";
import type { Promotion } from "../api/promotions";

export const samplePromotion: Promotion = {
  id: "10000000-0000-4000-8000-000000000001", code: "SAMPLE_25", percent: 25, enabled: false,
  revision: 3, starts_at: "2026-10-31T00:00:00Z", ends_at: null, max_uses: 100,
  plan_codes: ["lite", "pro"], reserved_count: 2, redeemed_count: 5,
  created_at: "2026-10-31T00:00:00Z", updated_at: "2026-10-31T00:00:00Z",
};
describe("promotion operator UI", () => {
  it("denies reads and form controls until the effective permission is loaded", () => {
    const request = vi.fn();
    const html = renderToStaticMarkup(<PromotionsScreen client={{ request }} adminTokenSet access={{ read: false, manage: false }} />);
    expect(html).toContain("Нет доступа к промокодам");
    expect(html).not.toContain("Создать промокод");
    expect(request).not.toHaveBeenCalled();
  });
  it("renders counts and bounded pages with no management button for read-only access", () => {
    const html = renderToStaticMarkup(<PromotionList items={[samplePromotion]} canManage={false} busy={false} offset={50} hasMore={false} onEdit={vi.fn()} onPage={vi.fn()} />);
    expect(html).toContain("SAMPLE_25");
    expect(html).toContain("В резерве: 2");
    expect(html).toContain("Использовано: 5");
    expect(html).toContain("Лимит: 100");
    expect(html).toContain("Страница 2");
    expect(html).not.toContain("Редактировать");
    expect(html).not.toContain(samplePromotion.id);
  });
  it("retains an immutable code and revision while editing and requires a separate confirmation", () => {
    const html = renderToStaticMarkup(<PromotionEditor existing={samplePromotion} locked={false} onPrepare={vi.fn()} onCancel={vi.fn()} />);
    expect(html).toMatch(/<input(?=[^>]*name="code")(?=[^>]*readOnly)[^>]*>/);
    expect(html).toContain("Версия 3");
    expect(html).toContain("Проверить изменения");
    expect(html).not.toContain('role="dialog"');
    expect(html).toContain("Окончание");
  });
  it("offers a disabled-by-default new draft without choosing a real campaign", () => {
    const html = renderToStaticMarkup(<PromotionEditor locked={false} onPrepare={vi.fn()} onCancel={vi.fn()} />);
    expect(html).toContain("Создать промокод");
    expect(html).toMatch(/name="code"[^>]*value=""/);
    expect(html).not.toMatch(/name="enabled"[^>]*checked/);
    expect(html).toContain("0 — без ограничений");
  });
  it("maps conflict and ambiguous outcomes without rendering backend data", () => {
    expect(promotionMutationError(new AdminApiError("admin_bad_request", "private token", 409))).toContain("Обновите список");
    expect(promotionMutationError(new Error("secret transport"))).not.toContain("secret");
    expect(promotionMutationError(new AdminApiError("admin_server_error", "private token", 503))).toContain("Повторите");
  });
});
