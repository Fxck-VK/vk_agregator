import { describe, expect, it, vi } from "vitest";
import { createAdminClient } from "./adminClient";
import {
  buildPromotionConfig, fetchPromotions, promotionAccess, preparePromotionMutation, savePromotion,
  type PromotionDraft,
} from "./promotions";

const valid: PromotionDraft = {
  code: " offer_25 ", percent: "25", enabled: false, startsAt: "2026-10-31T00:00:00Z",
  endsAt: "", maxUses: "0", planCodes: ["pro", "lite"],
};

describe("promotion configuration", () => {
  it("normalizes ASCII codes and freezes UTC config without introducing a campaign", () => {
    expect(buildPromotionConfig(valid)).toEqual({
      code: "OFFER_25", percent: 25, enabled: false, starts_at: "2026-10-31T00:00:00.000Z",
      ends_at: null, max_uses: 0, plan_codes: ["lite", "pro"],
    });
  });
  it.each([
    { code: "aßb" }, { code: "абв" }, { code: "ab" }, { code: "a/b" }, { code: "x".repeat(65) },
    { percent: "0" }, { percent: "100" }, { percent: "1.5" }, { percent: "1e1" },
    { maxUses: "-1" }, { maxUses: "9007199254740992" }, { planCodes: [] }, { planCodes: ["unknown"] },
    { startsAt: "2026-10-31T00:00" }, { startsAt: "2026-02-31T00:00:00Z" },
    { endsAt: "2026-10-31T00:00:00Z" }, { endsAt: "2026-10-30T23:59:59Z" },
  ])("rejects invalid config %j", (change) => {
    expect(() => buildPromotionConfig({ ...valid, ...change })).toThrow();
  });
  it("rejects changing an existing code or omitting its revision", () => {
    expect(() => buildPromotionConfig(valid, { code: "OLD_CODE", revision: 1 })).toThrow();
    expect(() => buildPromotionConfig(valid, { code: "OFFER_25", revision: 0 })).toThrow();
    expect(buildPromotionConfig(valid, { code: "OFFER_25", revision: 7 }).revision).toBe(7);
  });
});

describe("promotion permissions", () => {
  it.each(["owner", "admin", "finance"])("reads only the effective %s permissions", (role) => {
    expect(promotionAccess({ effective_role: role, roles: [{ role, permissions: ["promotions:read", "promotions:manage"] }] })).toEqual({ read: true, manage: true });
    expect(promotionAccess({ effective_role: role, roles: [{ role, permissions: ["promotions:read"] }] })).toEqual({ read: true, manage: false });
  });
  it.each([undefined, {}, { effective_role: "support", roles: [{ role: "owner", permissions: ["promotions:read", "promotions:manage"] }] }, { effective_role: "operator", roles: [] }])("fails closed for denied/missing contracts %j", (data) => {
    expect(promotionAccess(data)).toEqual({ read: false, manage: false });
  });
});

describe("protected promotion requests", () => {
  it("bounds list pagination and normalizes the empty Go slice", async () => {
    const request = vi.fn().mockResolvedValue({ items: null, has_more: false });
    expect(await fetchPromotions({ request }, 50)).toEqual({ items: [], has_more: false });
    expect(request.mock.calls[0][0]).toBe("/admin/promotions?limit=50&offset=50");
    await expect(fetchPromotions({ request }, -1)).rejects.toThrow();
    await expect(fetchPromotions({ request }, 0.5)).rejects.toThrow();
  });
  it("replays the exact immutable body and bare UUID after an ambiguous network result", async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockRejectedValueOnce(new Error("private transport details"))
      .mockResolvedValueOnce(new Response(JSON.stringify({ id: "10000000-0000-4000-8000-000000000001" })));
    const client = createAdminClient({ tokenProvider: () => "private-token", fetchImpl });
    const config = buildPromotionConfig(valid);
    const mutation = preparePromotionMutation(config);
    config.percent = 99;
    config.plan_codes.push("elite");
    expect(mutation.key).toMatch(/^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/i);
    await expect(savePromotion(client, mutation)).rejects.toMatchObject({ code: "admin_network_error" });
    await savePromotion(client, mutation);
    expect(fetchImpl).toHaveBeenCalledTimes(2);
    for (const call of fetchImpl.mock.calls) {
      expect(call[0]).toBe("/admin/promotions");
      expect((call[1]?.headers as Headers).get("X-Idempotency-Key")).toBe(mutation.key);
      expect(JSON.parse(call[1]?.body as string)).toMatchObject({ percent: 25, plan_codes: ["lite", "pro"] });
    }
  });
  it("uses only validated UUID update paths", () => {
    const config = buildPromotionConfig(valid, { code: "OFFER_25", revision: 2 });
    expect(preparePromotionMutation(config, "10000000-0000-4000-8000-000000000001").path).toBe("/admin/promotions/10000000-0000-4000-8000-000000000001");
    expect(() => preparePromotionMutation(config, "../users")).toThrow();
  });
});
